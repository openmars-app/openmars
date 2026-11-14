//! Scene-owned, source-count ship effects with bounded staging and replay.
use super::{
    fleet::{Fleet, Journey, Key, Status},
    flight,
    materials::{Parameters, ShipParticleMaterial},
    particles::{Kind, Pool, Sprite},
};
use bevy::{
    camera::visibility::{NoFrustumCulling, RenderLayers},
    light::{NotShadowCaster, NotShadowReceiver},
    mesh::MeshTag,
    prelude::*,
};
use std::collections::BTreeMap;
fn error(message: &str) -> String {
    message.into()
}
const KINDS: [Kind; 5] = [
    Kind::Trail,
    Kind::Hover,
    Kind::Flame,
    Kind::Core,
    Kind::Exhaust,
];
struct Group {
    kind: Kind,
    pool: Pool,
    entities: Vec<Entity>,
}
struct FlightFx {
    serial: u64,
    root: Entity,
    groups: Vec<Group>,
    last: Option<u64>,
}
impl FlightFx {
    fn new(world: &mut World, parent: Entity, serial: u64) -> Result<Self, String> {
        let quality = 1.;
        let root = world
            .spawn((Transform::IDENTITY, Visibility::Inherited, ChildOf(parent)))
            .id();
        let seed = (serial as u32) ^ (serial >> 32) as u32;
        let groups = KINDS
            .into_iter()
            .map(|kind| {
                Ok(Group {
                    kind,
                    pool: Pool::new(kind, quality, seed ^ kind.mode().wrapping_mul(0x9e3779b9))?,
                    entities: Vec::with_capacity(kind.capacity(quality)?),
                })
            })
            .collect::<Result<_, String>>()?;
        Ok(Self {
            serial,
            root,
            groups,
            last: None,
        })
    }
    fn complete(&self) -> bool {
        self.groups
            .iter()
            .all(|g| g.entities.len() == g.pool.capacity())
    }
    fn stage(
        &mut self,
        world: &mut World,
        mesh: &Handle<Mesh>,
        materials: &[Handle<ShipParticleMaterial>],
        budget: &mut usize,
    ) {
        for group in &mut self.groups {
            while *budget > 0 && group.entities.len() < group.pool.capacity() {
                let entity = world
                    .spawn((
                        Mesh3d(mesh.clone()),
                        MeshMaterial3d(materials[group.kind.mode() as usize].clone()),
                        MeshTag::new(0),
                        Transform::from_scale(Vec3::ZERO),
                        Visibility::Inherited,
                        NotShadowCaster,
                        NotShadowReceiver,
                        NoFrustumCulling,
                        ChildOf(self.root),
                    ))
                    .id();
                group.entities.push(entity);
                *budget -= 1;
            }
        }
    }
    fn sample(
        &mut self,
        world: &mut World,
        journey: &Journey,
        parking: flight::Parking,
        age: u64,
        world_step: u64,
        destination: (
            crate::space::config::BodyPose,
            crate::space::config::BodyPose,
        ),
    ) -> Result<(), String> {
        let path = &journey.path;
        let start = journey.start.ok_or("Cannot sample pending ship effects")?;
        let reconstruct = self
            .last
            .is_none_or(|last| age < last || age.saturating_sub(last) > 4);
        for group in &mut self.groups {
            if group.kind == Kind::Core {
                let sprite = Pool::core(age, world_step, journey.pose(start + age)?);
                for &entity in &group.entities {
                    write_sprite(world, entity, sprite, Quat::IDENTITY)?;
                }
                continue;
            }
            // Finite source lifetimes bound seeks independently of clock age:
            // the trail is entirely dead by234, hover by73 and landing by90.
            let (local, from, to, origin, normal) = if group.kind == Kind::Exhaust {
                let (origin, normal) = (path.landed.position, path.landing_normal);
                if age < flight::LANDING_EXHAUST {
                    group.pool.clear();
                    for &e in &group.entities {
                        write_sprite(world, e, Sprite::hidden(), Quat::IDENTITY)?;
                    }
                    continue;
                }
                let local = age - flight::LANDING_EXHAUST;
                (local, 0, local.min(90), origin, normal)
            } else {
                let start = if group.kind == Kind::Flame {
                    age.saturating_sub(24)
                } else {
                    0
                };
                let end = match group.kind {
                    Kind::Hover => age.min(73),
                    Kind::Trail => age.min(234),
                    _ => age.min(flight::COMPLETE),
                };
                (age, start, end, parking.pad.position, parking.normal)
            };
            if reconstruct {
                group.pool.clear();
            }
            let begin = if reconstruct {
                from
            } else {
                self.last.unwrap().saturating_add(1).saturating_sub(
                    if group.kind == Kind::Exhaust {
                        flight::LANDING_EXHAUST
                    } else {
                        0
                    },
                )
            };
            if local <= to {
                for at in begin..=to {
                    let pose = journey.pose(
                        start
                            + if group.kind == Kind::Exhaust {
                                at + flight::LANDING_EXHAUST
                            } else {
                                at
                            },
                    )?;
                    group.pool.step(at, pose, origin, normal)?;
                }
            } else {
                group.pool.clear();
            }
            for (&entity, mut sprite) in group.entities.iter().zip(group.pool.sprites()) {
                if group.kind == Kind::Exhaust && sprite.opacity > 0. {
                    sprite.position = destination
                        .1
                        .local_to_world(destination.0.world_to_local(sprite.position));
                }
                // The original exhaust uses world XY planes with Z spin;
                // its radial velocity, not its quad, follows the surface.
                let rotation = if group.kind == Kind::Exhaust {
                    Quat::from_rotation_z(sprite.rotation as f32)
                } else {
                    Quat::IDENTITY
                };
                write_sprite(world, entity, sprite, rotation)?;
            }
        }
        self.last = Some(age);
        Ok(())
    }
}
fn write_sprite(
    world: &mut World,
    entity: Entity,
    sprite: Sprite,
    rotation: Quat,
) -> Result<(), String> {
    let transform = Transform {
        translation: sprite.position.as_vec3(),
        rotation,
        scale: Vec3::splat(sprite.scale as f32),
    };
    if world
        .get::<Transform>(entity)
        .is_none_or(|old| *old != transform)
    {
        *world
            .get_mut::<Transform>(entity)
            .ok_or_else(|| error("ship particle entity absent"))? = transform;
    }
    let value = sprite.tag();
    if world
        .get::<MeshTag>(entity)
        .is_none_or(|tag| tag.value != value)
    {
        world
            .get_mut::<MeshTag>(entity)
            .ok_or_else(|| error("ship particle tag absent"))?
            .value = value;
    }
    Ok(())
}

#[derive(Default)]
pub struct Effects {
    mesh: Option<Handle<Mesh>>,
    materials: Vec<Handle<ShipParticleMaterial>>,
    warm: Vec<Entity>,
    entries: BTreeMap<Key, FlightFx>,
}
impl Effects {
    pub fn initialize(&mut self, world: &mut World, parent: Entity) -> bool {
        if self.mesh.is_some() {
            return true;
        }
        let Some(image) = crate::assets::texture(world, "textures/effects/smoke") else {
            return false;
        };
        let mesh = world
            .resource_mut::<Assets<Mesh>>()
            .add(Mesh::from(Rectangle::new(1., 1.)));
        for kind in KINDS {
            let material =
                world
                    .resource_mut::<Assets<ShipParticleMaterial>>()
                    .add(ShipParticleMaterial {
                        parameters: Parameters {
                            mode: UVec4::new(kind.mode(), 0, 0, 0),
                        },
                        image: image.clone(),
                    });
            self.warm.push(
                world
                    .spawn((
                        Mesh3d(mesh.clone()),
                        MeshMaterial3d(material.clone()),
                        MeshTag::new(512),
                        Transform::from_xyz((kind.mode() as f32 - 2.) * 0.3, 0., 0.)
                            .with_scale(Vec3::splat(0.2)),
                        Visibility::Inherited,
                        RenderLayers::layer(2),
                        NoFrustumCulling,
                        NotShadowCaster,
                        NotShadowReceiver,
                        ChildOf(parent),
                    ))
                    .id(),
            );
            self.materials.push(material);
        }
        self.mesh = Some(mesh);
        true
    }
    pub fn warm_nodes(&self) -> &[Entity] {
        &self.warm
    }
    pub fn finish_warm(&mut self, world: &mut World) {
        for e in self.warm.drain(..) {
            world.despawn(e);
        }
    }
    pub fn clear(&mut self, world: &mut World) {
        for (_, entry) in std::mem::take(&mut self.entries) {
            world.despawn(entry.root);
        }
        self.finish_warm(world);
        if let Some(mesh) = self.mesh.take() {
            world.resource_mut::<Assets<Mesh>>().remove(mesh.id());
        }
        for m in self.materials.drain(..) {
            world
                .resource_mut::<Assets<ShipParticleMaterial>>()
                .remove(m.id());
        }
    }
    pub fn stage(
        &mut self,
        world: &mut World,
        parent: Entity,
        fleet: &mut Fleet,
        step: u64,
        quiet: bool,
    ) -> Result<(), String> {
        self.entries.retain(|key, fx| {
            let keep = !quiet
                && fleet
                    .ships
                    .get(key)
                    .is_some_and(|s| s.status == Status::Flying && s.serial == fx.serial);
            if !keep {
                world.despawn(fx.root);
            }
            keep
        });
        if quiet {
            return Ok(());
        }
        for (key, ship) in &mut fleet.ships {
            if ship.status != Status::Flying || self.entries.contains_key(key) {
                continue;
            }
            let entry = FlightFx::new(world, parent, ship.serial)?;
            ship.journey.as_mut().ok_or("Missing staged flight")?.start = None;
            self.entries.insert(key.clone(), entry);
        }
        let mut budget = 128;
        for (key, entry) in &mut self.entries {
            entry.stage(
                world,
                self.mesh.as_ref().ok_or("Missing ship particle mesh")?,
                &self.materials,
                &mut budget,
            );
            let ship = fleet.ships.get_mut(key).ok_or("Missing effect hull")?;
            let journey = ship.journey.as_mut().ok_or("Missing effect journey")?;
            if journey.start.is_none() && entry.complete() {
                // Resource staging must not shorten the source flight or miss
                // a moving destination: predict arrival from actual activation.
                **journey = Journey::new(journey.path.parked, journey.landing.body, step)?;
            }
        }
        Ok(())
    }
    pub fn sample(&mut self, world: &mut World, fleet: &Fleet, step: u64) -> Result<(), String> {
        self.entries.retain(|key, entry| {
            let keep = fleet
                .ships
                .get(key)
                .is_some_and(|s| s.status == Status::Flying && s.serial == entry.serial);
            if !keep {
                world.despawn(entry.root);
            }
            keep
        });
        for (key, entry) in &mut self.entries {
            let ship = fleet.ships.get(key).ok_or("Missing effect ship")?;
            let journey = ship.journey.as_ref().ok_or("Missing effect path")?;
            let Some(start) = journey.start else {
                continue;
            };
            if !entry.complete() || ship.status != Status::Flying {
                continue;
            }
            let arrival = (start + flight::COMPLETE) as f64 / 60.;
            let now = step as f64 / 60.;
            let body = journey.landing.body;
            let destination = (
                crate::space::config::in_mars_frame(
                    crate::space::config::body_pose(body, arrival)?,
                    arrival,
                )?,
                crate::space::config::in_mars_frame(
                    crate::space::config::body_pose(body, now)?,
                    now,
                )?,
            );
            let mut parking = ship.parking;
            parking.normal = journey.path.start_normal;
            parking.pad.position = journey.path.parked.position - parking.normal * 0.015;
            entry.sample(
                world,
                journey,
                parking,
                step.saturating_sub(start),
                step,
                destination,
            )?;
        }
        Ok(())
    }
    pub fn diagnostics(&self, world: &World) -> serde_json::Value {
        serde_json::json!({"warmNodes":self.warm.len(),"materials":self.materials.len(),"entries":self.entries.iter().map(|(key,e)|serde_json::json!({"owner":key.owner,"ordinal":key.ordinal,"serial":e.serial,"complete":e.complete(),"last":e.last,"groups":e.groups.iter().map(|g|serde_json::json!({"kind":format!("{:?}",g.kind),"capacity":g.pool.capacity(),"instances":g.entities.len(),"visible":g.entities.iter().filter(|e|world.get::<MeshTag>(**e).is_some_and(|tag|tag.value&1023>0)).count()})).collect::<Vec<_>>()})).collect::<Vec<_>>()})
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn effects_stage_before_activation_keep_source_counts_follow_the_destination_and_retire() {
        let mut world = World::new();
        world.insert_resource(Assets::<Mesh>::default());
        world.insert_resource(Assets::<ShipParticleMaterial>::default());
        let root = world
            .spawn((Transform::IDENTITY, Visibility::Inherited))
            .id();
        let mut effects = Effects {
            mesh: Some(Handle::default()),
            materials: vec![Handle::default(); 5],
            ..default()
        };
        let key = Key {
            owner: "p".into(),
            ordinal: 0,
        };
        let park = flight::parking(1, 2.02).unwrap()[0];
        let mut fleet = Fleet::default();
        fleet.order.push(key.clone());
        fleet.ships.insert(
            key.clone(),
            super::super::fleet::Ship {
                parking: park,
                status: Status::Flying,
                journey: Some(Box::new(
                    Journey::new(park.ship, openmars_scene::view::BodyId::Venus, 0).unwrap(),
                )),
                landing: None,
                trade: Some("colony".into()),
                pose: park.ship,
                serial: 1,
            },
        );
        // Bevy0.20 resources also have entities. Compare with the real owner
        // baseline rather than assuming a bare World contains only the root.
        let baseline = world.query::<Entity>().iter(&world).count();
        let mut previous = 0;
        for step in 100..=106 {
            effects
                .stage(&mut world, root, &mut fleet, step, false)
                .unwrap();
            let entry = &effects.entries[&key];
            let nodes: usize = entry.groups.iter().map(|g| g.entities.len()).sum();
            assert!(nodes - previous <= 128);
            previous = nodes;
            if step < 106 {
                assert!(fleet.ships[&key].journey.as_ref().unwrap().start.is_none());
                fleet.advance(step).unwrap();
                assert_eq!(fleet.ships[&key].pose, park.ship);
            }
        }
        assert_eq!(previous, 781);
        let entry = &effects.entries[&key];
        assert_eq!(
            entry
                .groups
                .iter()
                .map(|g| g.pool.capacity())
                .collect::<Vec<_>>(),
            vec![600, 100, 60, 1, 20]
        );
        assert_eq!(fleet.ships[&key].journey.as_ref().unwrap().start, Some(106));
        fleet.advance(146).unwrap();
        effects.sample(&mut world, &fleet, 146).unwrap();
        let counts: Vec<_> = effects.entries[&key]
            .groups
            .iter()
            .map(|g| {
                g.entities
                    .iter()
                    .filter(|e| world.get::<MeshTag>(**e).unwrap().value & 1023 > 0)
                    .count()
            })
            .collect();
        assert!(counts[1] > 0 && counts[2] > 0 && counts[3] == 1);
        let tags: Vec<_> = effects.entries[&key]
            .groups
            .iter()
            .flat_map(|g| {
                g.entities
                    .iter()
                    .map(|e| world.get::<MeshTag>(*e).unwrap().value)
            })
            .collect();
        effects.sample(&mut world, &fleet, 146).unwrap();
        assert_eq!(
            tags,
            effects.entries[&key]
                .groups
                .iter()
                .flat_map(|g| g
                    .entities
                    .iter()
                    .map(|e| world.get::<MeshTag>(*e).unwrap().value))
                .collect::<Vec<_>>()
        );
        let step = 106 + flight::LANDING_EXHAUST + 20;
        fleet.advance(step).unwrap();
        effects.sample(&mut world, &fleet, step).unwrap();
        let body = crate::space::config::in_mars_frame(
            crate::space::config::body_pose(openmars_scene::view::BodyId::Venus, step as f64 / 60.)
                .unwrap(),
            step as f64 / 60.,
        )
        .unwrap();
        let exhaust = &effects.entries[&key].groups[4];
        let visible: Vec<_> = exhaust
            .entities
            .iter()
            .filter(|e| world.get::<MeshTag>(**e).unwrap().value & 1023 > 0)
            .collect();
        assert!(!visible.is_empty());
        for e in visible {
            let position = world.get::<Transform>(*e).unwrap().translation.as_dvec3();
            assert!(
                ((position - body.center).length() - 2.02).abs() < 0.15,
                "landing exhaust stays on the current moving Venus"
            );
        }
        fleet.advance(106 + flight::COMPLETE).unwrap();
        effects
            .sample(&mut world, &fleet, 106 + flight::COMPLETE)
            .unwrap();
        assert!(effects.entries.is_empty());
        assert_eq!(world.query::<Entity>().iter(&world).count(), baseline);
        effects.clear(&mut world);
        assert!(effects.materials.is_empty() && effects.mesh.is_none());
    }
}
