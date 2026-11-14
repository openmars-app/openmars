//! Public presentation transitions with stable hull identity and moving destinations.
//! Availability alone never invents a trade, destination or departure animation.
use super::flight::{self, Destination, FlightPath, Parking, Pose};
use crate::space::config;
use bevy::math::{DQuat, DVec3};
use openmars_scene::{ScenePlayer, SceneSnapshot, offworld::OffworldScene, view::BodyId};
use std::collections::{BTreeMap, BTreeSet};

#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord)]
pub struct Key {
    pub owner: String,
    pub ordinal: u32,
}
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Status {
    Parked,
    Unavailable,
    Flying,
    Landed,
}
#[derive(Clone, Debug)]
pub struct Landing {
    pub body: BodyId,
    pub position: DVec3,
    pub rotation: DQuat,
}
impl Landing {
    fn pose(&self, seconds: f64) -> Result<Pose, String> {
        let body = config::in_mars_frame(config::body_pose(self.body, seconds)?, seconds)?;
        Ok(Pose {
            position: body.local_to_world(self.position),
            rotation: body.rotation * self.rotation,
        })
    }
}
#[derive(Clone, Debug)]
pub struct Journey {
    pub path: FlightPath,
    pub start: Option<u64>,
    pub landing: Landing,
}
impl Journey {
    pub(super) fn new(parked: Pose, body: BodyId, step: u64) -> Result<Self, String> {
        let arrival = step
            .checked_add(flight::COMPLETE)
            .ok_or("Flight deadline overflow")? as f64
            / 60.;
        // The original path is authored in board-local coordinates. Predict its
        // endpoint in the arrival Mars frame; rotating a fixed inertial departure
        // frame would let Mars move through the ship during the lift-off phase.
        let arrival_frame = config::body_pose(BodyId::Mars, arrival)?;
        let destination = config::body_pose(body, arrival)?;
        let center = arrival_frame.world_to_local(destination.center);
        let normal = -center.normalize();
        let position = center + normal * config::config(body).radius;
        let path = FlightPath::new(
            parked,
            Destination {
                position,
                normal,
                center,
            },
        )?;
        let landing = Landing {
            body,
            position: destination.world_to_local(arrival_frame.local_to_world(position)),
            rotation: destination.rotation.inverse()
                * arrival_frame.rotation
                * path.landed.rotation,
        };
        Ok(Self {
            path,
            start: Some(step),
            landing,
        })
    }
    pub(super) fn pose(&self, step: u64) -> Result<Pose, String> {
        let start = self.start.unwrap_or(step);
        let age = step.saturating_sub(start);
        let source = self.path.sample(age)?;
        if age <= flight::ARC_END {
            return Ok(source);
        }
        let arrival = (start + flight::COMPLETE) as f64 / 60.;
        let now = step as f64 / 60.;
        let body = self.landing.body;
        let target = config::in_mars_frame(config::body_pose(body, arrival)?, arrival)?;
        let current = config::in_mars_frame(config::body_pose(body, now)?, now)?;
        let t = ((age - flight::ARC_END) as f64 / (flight::CRUISE_END - flight::ARC_END) as f64)
            .min(1.);
        let blend = t * t * (3. - 2. * t);
        Ok(Pose {
            position: source.position.lerp(
                current.local_to_world(target.world_to_local(source.position)),
                blend,
            ),
            rotation: source.rotation.slerp(
                current.rotation * target.rotation.inverse() * source.rotation,
                blend,
            ),
        })
    }
}
#[derive(Clone, Debug)]
pub struct Ship {
    pub parking: Parking,
    pub status: Status,
    pub journey: Option<Box<Journey>>,
    pub landing: Option<Landing>,
    pub trade: Option<String>,
    pub pose: Pose,
    pub serial: u64,
}
impl Ship {
    fn new(parking: Parking) -> Self {
        Self {
            parking,
            status: Status::Parked,
            journey: None,
            landing: None,
            trade: None,
            pose: parking.ship,
            serial: 0,
        }
    }
    fn park(&mut self) {
        self.status = Status::Parked;
        self.journey = None;
        self.landing = None;
        self.trade = None;
        self.pose = self.parking.ship;
    }
    fn trade(
        &mut self,
        id: &str,
        body: Option<BodyId>,
        step: u64,
        live: bool,
        serial: u64,
    ) -> Result<(), String> {
        self.trade = Some(id.into());
        self.serial = serial;
        let Some(body) = body.filter(|body| *body != BodyId::Mars) else {
            // Unknown public locations have no authored visual destination.
            self.status = Status::Unavailable;
            return Ok(());
        };
        let journey = Journey::new(self.parking.ship, body, step)?;
        if live {
            self.pose = self.parking.ship;
            self.status = Status::Flying;
            self.journey = Some(Box::new(journey));
        } else {
            // Quiet history anchors to the actual body now, without a future flight.
            let current = config::body_pose(body, step as f64 / 60.)?;
            let mars = config::body_pose(BodyId::Mars, step as f64 / 60.)?;
            let normal = (mars.center - current.center).normalize();
            let local_normal = current.rotation.inverse() * normal;
            let landed = FlightPath::new(
                self.parking.ship,
                Destination {
                    position: mars
                        .world_to_local(current.center + normal * config::config(body).radius),
                    normal: mars.rotation.inverse() * normal,
                    center: mars.world_to_local(current.center),
                },
            )?
            .landed;
            self.landing = Some(Landing {
                body,
                position: local_normal * config::config(body).radius,
                rotation: current.rotation.inverse() * mars.rotation * landed.rotation,
            });
            self.status = Status::Landed;
            self.pose = self.landing.as_ref().unwrap().pose(step as f64 / 60.)?;
        }
        Ok(())
    }
}
#[derive(Default)]
pub struct Fleet {
    pub ships: BTreeMap<Key, Ship>,
    pub order: Vec<Key>,
    previous: Option<(u32, Vec<ScenePlayer>, OffworldScene)>,
    quiet: bool,
    serial: u64,
    last_step: Option<u64>,
}
impl Fleet {
    pub fn reconcile(
        &mut self,
        scene: &SceneSnapshot,
        quiet: bool,
        step: u64,
    ) -> Result<(), String> {
        let unchanged = self.previous.as_ref().is_some_and(|p| {
            p.0 == scene.generation && p.1 == scene.players && p.2 == scene.offworld
        });
        if unchanged && (!quiet || self.quiet) {
            self.quiet = quiet;
            return Ok(());
        }
        let settle = quiet
            || self
                .previous
                .as_ref()
                .is_none_or(|p| p.0 != scene.generation);
        let capacities: Vec<_> = scene
            .players
            .iter()
            .map(|p| {
                let fleet = scene.offworld.fleets.iter().find(|f| f.player_id == p.id);
                (
                    p.id.clone(),
                    if scene.offworld.fleets_enabled {
                        fleet.map_or(1, |f| f.total)
                    } else {
                        0
                    },
                    if scene.offworld.fleets_enabled {
                        fleet.map(|f| f.available)
                    } else {
                        None
                    },
                )
            })
            .collect();
        let order: Vec<_> = capacities
            .iter()
            .flat_map(|(owner, total, _)| {
                (0..*total).map(|ordinal| Key {
                    owner: owner.clone(),
                    ordinal,
                })
            })
            .collect();
        let keys: BTreeSet<_> = order.iter().cloned().collect();
        self.ships.retain(|key, _| keys.contains(key));
        for (key, parking) in order.iter().zip(flight::parking(order.len(), 2.02)?) {
            let ship = self
                .ships
                .entry(key.clone())
                .or_insert_with(|| Ship::new(parking));
            ship.parking = parking;
            if settle || (ship.status == Status::Unavailable && ship.trade.is_none()) {
                ship.park();
            }
            if ship.trade.as_ref().is_some_and(|id| {
                !scene.offworld.colonies.iter().any(|c| {
                    &c.id == id && c.traded_this_gen && c.trader_id.as_ref() == Some(&key.owner)
                })
            }) {
                ship.park();
            }
            if ship.status == Status::Parked {
                ship.pose = parking.ship;
            }
        }
        self.order = order;
        for trade in scene.offworld.colonies.iter().filter(|c| c.traded_this_gen) {
            if self
                .ships
                .values()
                .any(|s| s.trade.as_ref() == Some(&trade.id))
            {
                continue;
            }
            let Some(key) = self
                .order
                .iter()
                .find(|key| {
                    trade.trader_id.as_ref() == Some(&key.owner)
                        && self.ships[*key].status == Status::Parked
                })
                .cloned()
            else {
                continue;
            };
            let edge = self.previous.as_ref().is_some_and(|p| {
                p.2.colonies
                    .iter()
                    .any(|old| old.id == trade.id && !old.traded_this_gen)
            });
            self.serial = self
                .serial
                .checked_add(1)
                .ok_or("Fleet presentation serial overflow")?;
            self.ships.get_mut(&key).unwrap().trade(
                &trade.id,
                BodyId::from_location(&trade.location),
                step,
                !settle && edge,
                self.serial,
            )?;
        }
        for (owner, total, available) in capacities {
            let Some(available) = available else {
                continue;
            };
            let used = self
                .ships
                .iter()
                .filter(|(key, s)| key.owner == owner && s.status != Status::Parked)
                .count();
            let mut missing = (total - available) as usize;
            missing = missing.saturating_sub(used);
            for key in self.order.iter().rev().filter(|k| k.owner == owner) {
                let ship = self.ships.get_mut(key).unwrap();
                if missing > 0 && ship.status == Status::Parked {
                    ship.status = Status::Unavailable;
                    missing -= 1;
                }
            }
        }
        self.previous = Some((
            scene.generation,
            scene.players.clone(),
            scene.offworld.clone(),
        ));
        self.quiet = quiet;
        if settle {
            self.last_step = None;
        }
        Ok(())
    }
    pub fn advance(&mut self, step: u64) -> Result<(), String> {
        if self.last_step.is_some_and(|last| step < last) {
            return Err("Fleet clock regressed without settlement".into());
        }
        self.last_step = Some(step);
        for ship in self.ships.values_mut() {
            if ship.status == Status::Flying {
                let journey = ship.journey.as_ref().ok_or("Flying hull has no journey")?;
                let Some(start) = journey.start else {
                    ship.pose = journey.path.parked;
                    continue;
                };
                if step.saturating_sub(start) >= flight::COMPLETE {
                    ship.landing = Some(journey.landing.clone());
                    ship.status = Status::Landed;
                } else {
                    ship.pose = journey.pose(step)?;
                }
            }
            if ship.status == Status::Landed {
                ship.pose = ship
                    .landing
                    .as_ref()
                    .ok_or("Landed hull has no anchor")?
                    .pose(step as f64 / 60.)?;
            }
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use openmars_scene::offworld::{SceneColony, SceneFleet};
    fn scene() -> SceneSnapshot {
        SceneSnapshot {
            game_id: "fleet-test".into(),
            map_id: "tharsis".into(),
            generation: 2,
            temperature: -30,
            oxygen: 0,
            oceans: 0,
            max_oceans: 9,
            venus: 0,
            tiles: vec![],
            players: vec![ScenePlayer {
                id: "p".into(),
                color: "#ff0000".into(),
            }],
            selected_tile: None,
            selectable_tiles: vec![],
            offworld: OffworldScene {
                fleets_enabled: true,
                colonies: vec![SceneColony {
                    id: "Titan".into(),
                    location: "venus".into(),
                    traded_this_gen: false,
                    trader_id: None,
                }],
                fleets: vec![SceneFleet {
                    player_id: "p".into(),
                    total: 2,
                    available: 2,
                }],
                station: None,
            },
        }
    }
    fn trade(scene: &mut SceneSnapshot) {
        scene.offworld.colonies[0].traded_this_gen = true;
        scene.offworld.colonies[0].trader_id = Some("p".into());
        scene.offworld.fleets[0].available -= 1;
    }
    fn key(ordinal: u32) -> Key {
        Key {
            owner: "p".into(),
            ordinal,
        }
    }
    fn on_body(ship: &Ship, body: BodyId, step: u64) {
        let body_pose = config::in_mars_frame(
            config::body_pose(body, step as f64 / 60.).unwrap(),
            step as f64 / 60.,
        )
        .unwrap();
        assert!(
            ((ship.pose.position - body_pose.center).length() - config::config(body).radius).abs()
                < 1e-10
        );
    }
    #[test]
    fn disabled_colonies_own_no_ships_and_reenabling_settles_existing_trades() {
        let mut s = scene();
        let mut fleet = Fleet::default();
        fleet.reconcile(&s, true, 0).unwrap();
        trade(&mut s);
        fleet.reconcile(&s, false, 1).unwrap();
        fleet.advance(1).unwrap();
        assert_eq!(fleet.ships[&key(0)].status, Status::Flying);
        s.offworld.fleets_enabled = false;
        fleet.reconcile(&s, false, 2).unwrap();
        fleet.advance(2).unwrap();
        assert!(fleet.ships.is_empty() && fleet.order.is_empty());
        assert_eq!(
            s.offworld.fleets[0].total, 2,
            "visibility must not rewrite public capacities"
        );
        s.offworld.fleets_enabled = true;
        fleet.reconcile(&s, false, 3).unwrap();
        fleet.advance(3).unwrap();
        assert_eq!(fleet.ships.len(), 2);
        assert_eq!(fleet.ships[&key(0)].status, Status::Landed);
        assert!(fleet.ships[&key(0)].journey.is_none());
    }
    #[test]
    fn quiet_public_history_follows_its_actual_location_and_unavailability_invents_no_flight() {
        let mut s = scene();
        trade(&mut s);
        s.offworld.fleets[0].available = 0;
        let mut fleet = Fleet::default();
        fleet.reconcile(&s, true, 500).unwrap();
        fleet.advance(500).unwrap();
        let ship = &fleet.ships[&key(0)];
        assert_eq!(ship.status, Status::Landed);
        assert_eq!(
            ship.landing.as_ref().unwrap().body,
            BodyId::Venus,
            "colony name cannot imply Saturn"
        );
        assert!(ship.journey.is_none());
        on_body(ship, BodyId::Venus, 500);
        assert_eq!(fleet.ships[&key(1)].status, Status::Unavailable);
        fleet.advance(1600).unwrap();
        on_body(&fleet.ships[&key(0)], BodyId::Venus, 1600);
        assert!(fleet.ships.values().all(|s| s.journey.is_none()));
    }
    #[test]
    fn live_edges_survive_roster_capacity_color_and_unrelated_snapshots_without_restarting() {
        let mut s = scene();
        let mut fleet = Fleet::default();
        fleet.reconcile(&s, true, 100).unwrap();
        fleet.advance(100).unwrap();
        trade(&mut s);
        fleet.reconcile(&s, false, 101).unwrap();
        fleet.advance(101).unwrap();
        let original = fleet.ships[&key(0)].clone();
        assert_eq!(original.status, Status::Flying);
        assert!(
            original
                .pose
                .position
                .abs_diff_eq(original.parking.ship.position, 1e-12)
        );
        assert!(
            original
                .pose
                .rotation
                .abs_diff_eq(original.parking.ship.rotation, 1e-12)
        );
        fleet.advance(250).unwrap();
        let moved = fleet.ships[&key(0)].pose;
        assert_ne!(moved, original.pose);
        s.offworld.fleets[0].total = 3;
        s.offworld.fleets[0].available = 2;
        s.players[0].color = "#00ff00".into();
        fleet.reconcile(&s, false, 250).unwrap();
        fleet.advance(250).unwrap();
        let current = &fleet.ships[&key(0)];
        assert_eq!(current.pose, moved);
        assert_eq!(current.serial, original.serial);
        assert_eq!(current.journey.as_ref().unwrap().start, Some(101));
        assert_eq!(fleet.ships.len(), 3);
        fleet.advance(101 + flight::COMPLETE).unwrap();
        assert_eq!(fleet.ships[&key(0)].status, Status::Landed);
        on_body(&fleet.ships[&key(0)], BodyId::Venus, 101 + flight::COMPLETE);
        assert!(fleet.advance(100).is_err());
    }
    #[test]
    fn reduced_motion_reconnect_and_generation_reset_settle_without_replaying() {
        let mut s = scene();
        let mut fleet = Fleet::default();
        fleet.reconcile(&s, true, 0).unwrap();
        trade(&mut s);
        fleet.reconcile(&s, false, 10).unwrap();
        fleet.advance(30).unwrap();
        assert_eq!(fleet.ships[&key(0)].status, Status::Flying);
        fleet.reconcile(&s, true, 30).unwrap();
        fleet.advance(30).unwrap();
        assert_eq!(fleet.ships[&key(0)].status, Status::Landed);
        assert!(fleet.ships[&key(0)].journey.is_none());
        fleet.reconcile(&s, false, 30).unwrap();
        fleet.advance(40).unwrap();
        assert_eq!(fleet.ships[&key(0)].status, Status::Landed);
        s.generation += 1;
        s.offworld.colonies[0].traded_this_gen = false;
        s.offworld.colonies[0].trader_id = None;
        s.offworld.fleets[0].available = 2;
        fleet.reconcile(&s, false, 41).unwrap();
        fleet.advance(41).unwrap();
        assert!(fleet.ships.values().all(|s| s.status == Status::Parked));
        s.offworld.colonies.push(SceneColony {
            id: "new-history".into(),
            location: "moon".into(),
            traded_this_gen: true,
            trader_id: Some("p".into()),
        });
        fleet.reconcile(&s, false, 42).unwrap();
        assert_eq!(
            fleet.ships[&key(0)].status,
            Status::Landed,
            "a first-seen used colony is history, not a false-to-true edge"
        );
        s.offworld.fleets.clear();
        fleet.reconcile(&s, true, 0).unwrap();
        fleet.advance(0).unwrap();
        assert_eq!(
            fleet.ships.len(),
            1,
            "original public-player fallback is one hull"
        );
    }
    #[test]
    fn unknown_locations_and_historical_traders_never_fabricate_destinations() {
        let mut s = scene();
        let mut fleet = Fleet::default();
        trade(&mut s);
        s.offworld.colonies[0].trader_id = Some("departed-player".into());
        fleet.reconcile(&s, true, 0).unwrap();
        assert_eq!(fleet.ships[&key(1)].status, Status::Unavailable);
        s.offworld.colonies[0].trader_id = Some("p".into());
        s.offworld.colonies[0].location = "unknown".into();
        fleet.reconcile(&s, true, 1).unwrap();
        assert!(
            fleet
                .ships
                .values()
                .all(|s| s.journey.is_none() && s.landing.is_none())
        );
        assert_eq!(fleet.ships[&key(0)].status, Status::Unavailable);
    }
    #[test]
    fn source_liftoff_remains_radial_and_above_the_moving_mars_surface() {
        let parking = flight::parking(3, 2.02).unwrap();
        for start in [0, 18000, 179999] {
            for park in &parking {
                let journey = Journey::new(park.ship, BodyId::Venus, start).unwrap();
                for age in 0..=flight::LIFTOFF {
                    let pose = journey.pose(start + age).unwrap();
                    let source = journey.path.sample(age).unwrap();
                    assert_eq!(
                        pose, source,
                        "source path belongs to the moving board frame"
                    );
                    assert!(pose.position.length() >= park.ship.position.length() - 1e-12);
                    assert!(pose.position.normalize().abs_diff_eq(park.normal, 1e-12));
                }
                for age in flight::LIFTOFF..=flight::ARC_END {
                    assert!(journey.pose(start + age).unwrap().position.length() > 2.02);
                }
            }
        }
    }
    #[test]
    fn final_approach_tracks_the_current_body_and_preserves_source_radial_descent() {
        let park = flight::parking(1, 2.02).unwrap()[0].ship;
        for start in [0, 18000, 179999] {
            for body in config::BODIES.into_iter().filter(|b| *b != BodyId::Mars) {
                let journey = Journey::new(park, body, start).unwrap();
                for age in [
                    flight::CRUISE_END,
                    flight::CRUISE_END + 30,
                    flight::CRUISE_END + 120,
                    flight::COMPLETE,
                ] {
                    let seconds = (start + age) as f64 / 60.;
                    let center =
                        config::in_mars_frame(config::body_pose(body, seconds).unwrap(), seconds)
                            .unwrap()
                            .center;
                    let pose = journey.pose(start + age).unwrap();
                    let t = (age - flight::CRUISE_END) as f64
                        / (flight::COMPLETE - flight::CRUISE_END) as f64;
                    assert!(
                        ((pose.position - center).length()
                            - config::config(body).radius
                            - (1. - t.powi(3)))
                        .abs()
                            < 1e-10,
                        "{body:?} must descend above the actual moving surface"
                    );
                }
                for edge in [flight::ARC_END, flight::CRUISE_END] {
                    let a = journey.pose(start + edge).unwrap();
                    let before = journey.pose(start + edge - 1).unwrap();
                    let after = journey.pose(start + edge + 1).unwrap();
                    assert!(
                        a.position.distance(before.position) < 3.
                            && a.position.distance(after.position) < 3.,
                        "{body:?} transition has no positional jump"
                    );
                }
            }
        }
    }
    #[test]
    fn predicted_landings_are_continuous_in_the_rotating_frame_and_keep_the_real_body_radius() {
        let park = flight::parking(1, 2.02).unwrap()[0].ship;
        for start in [0, 18000, 179999] {
            for body in config::BODIES.into_iter().filter(|b| *b != BodyId::Mars) {
                let journey = Journey::new(park, body, start).unwrap();
                assert!(
                    journey
                        .pose(start)
                        .unwrap()
                        .position
                        .abs_diff_eq(park.position, 1e-10)
                );
                let arrival = start + flight::COMPLETE;
                let flying = journey.pose(arrival).unwrap();
                let landed = journey.landing.pose(arrival as f64 / 60.).unwrap();
                assert!(
                    flying.position.abs_diff_eq(landed.position, 1e-10),
                    "{body:?} must meet the moving target"
                );
                assert!(flying.rotation.abs_diff_eq(landed.rotation, 1e-10));
                let body_pose = config::in_mars_frame(
                    config::body_pose(body, arrival as f64 / 60.).unwrap(),
                    arrival as f64 / 60.,
                )
                .unwrap();
                assert!(
                    ((landed.position - body_pose.center).length() - config::config(body).radius)
                        .abs()
                        < 1e-10
                );
                let before = journey.pose(arrival - 1).unwrap();
                assert!(
                    before.position.distance(landed.position) < 0.1,
                    "{body:?} no terminal jump"
                );
            }
        }
        assert!(Journey::new(park, BodyId::Venus, u64::MAX).is_err());
    }
}
