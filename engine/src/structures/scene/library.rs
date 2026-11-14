//! Prepare shared city materials with the real indexed vertex layout, without
//! creating a game plot or consuming a future city's animation clock.
use super::*;

pub(super) struct WarmLibrary {
    root: Entity,
    camera: Entity,
    pub(super) mesh: Handle<Mesh>,
    group: u32,
    frames: u32,
}

impl StructureRuntime {
    pub(super) fn retire_library(&mut self, world: &mut World) {
        if let Some(warm) = self.library_warm.take() {
            world.despawn(warm.root);
            world.despawn(warm.camera);
            self.group_pool.as_mut().unwrap().release(warm.group);
            world.resource_mut::<Assets<Mesh>>().remove(warm.mesh.id());
            self.retired_meshes.push(warm.mesh.id());
            self.library_retiring = Some(warm.mesh.id());
        }
    }
    pub(super) fn prepare_library(
        &mut self,
        world: &mut World,
        target: Handle<Image>,
        uploaded: bool,
    ) -> Result<bool, String> {
        if self.library_warmed {
            // Live chunk replacement can retire an old mesh while its successor
            // is still pending activation. Only our temporary preparation mesh
            // belongs to this barrier; waiting for every retired chunk deadlocks
            // that activation and prevents the old mesh from being released.
            if self
                .library_retiring
                .is_some_and(|id| self.retired_meshes.contains(&id))
            {
                return Ok(false);
            }
            self.library_retiring = None;
            return Ok(true);
        }
        if let Some(warm) = &mut self.library_warm {
            warm.frames += 1;
            if uploaded && warm.frames >= 2 {
                self.retire_library(world);
                self.library_warmed = true;
            }
            return Ok(false);
        }
        // Resolve every fallible material before creating owned view entities.
        let keys = (0..geometry::MATERIALS)
            .map(|index| MaterialKey {
                index,
                bright: false,
            })
            .chain((0..3).map(|index| MaterialKey {
                index,
                bright: true,
            }));
        let parts = keys
            .map(|key| self.material(world, key).map(|m| (key, m)))
            .collect::<Result<Vec<_>, _>>()?;
        let group = self.group_pool.as_mut().unwrap().allocate()?;
        let batch = geometry::MeshBatch {
            material: 0,
            positions: vec![
                [-0.3, -0.3, 0.],
                [0.3, -0.3, 0.],
                [0.3, 0.3, 0.],
                [-0.3, -0.3, 0.],
                [0.3, 0.3, 0.],
                [-0.3, 0.3, 0.],
            ],
            normals: vec![[0., 0., 1.]; 6],
            uv: vec![[0., 0.], [1., 0.], [1., 1.], [0., 0.], [1., 1.], [0., 1.]],
            colors: vec![[1.; 4]; 6],
            frost: vec![1.; 6],
            pivots: vec![[0.; 3]; 6],
            collapse: vec![[0.; 4]; 6],
        };
        let mesh = match batch.indexed(group) {
            Ok(indexed) => world.resource_mut::<Assets<Mesh>>().add(indexed.mesh),
            Err(_) => {
                self.group_pool.as_mut().unwrap().release(group);
                return Err(err("Structure preparation mesh"));
            }
        };
        self.group_pool.as_mut().unwrap().set(
            group,
            [
                [0.; 4],
                [0.; 4],
                [-1000., 1e9, 1., 0.],
                [0., 0., 1., 0.],
                [1., 0., 0., 0.],
                [0., 1., 0., 0.],
            ],
        );
        let root = world
            .spawn((Transform::default(), Visibility::Inherited))
            .id();
        for (index, (key, material)) in parts.into_iter().enumerate() {
            let mut entity = world.spawn((
                Mesh3d(mesh.clone()),
                MeshMaterial3d(material),
                Transform::from_xyz((index % 5) as f32 - 2., (index / 5) as f32 - 2., 0.),
                RenderLayers::layer(2),
                NoFrustumCulling,
                ChildOf(root),
            ));
            if [10, 14, 15].contains(&key.index) {
                entity.insert((NotShadowCaster, NotShadowReceiver));
            }
        }
        let camera = world
            .spawn((
                Camera3d::default(),
                bevy::core_pipeline::prepass::DepthPrepass,
                Camera {
                    order: -4,
                    ..default()
                },
                RenderTarget::Image(target.into()),
                Msaa::Off,
                Tonemapping::None,
                RenderLayers::layer(2),
                Transform::from_xyz(0., 0., 6.8).looking_at(Vec3::ZERO, Vec3::Y),
            ))
            .id();
        self.library_warm = Some(WarmLibrary {
            root,
            camera,
            mesh,
            group,
            frames: 0,
        });
        Ok(false)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn shared_preparation_does_not_block_live_chunk_retirement() {
        let mut world = World::new();
        world.init_resource::<Assets<Mesh>>();
        let temporary = world.resource_mut::<Assets<Mesh>>().add(Cuboid::default());
        let live = world.resource_mut::<Assets<Mesh>>().add(Cuboid::default());
        let mut runtime = StructureRuntime {
            library_warmed: true,
            library_retiring: Some(temporary.id()),
            retired_meshes: vec![temporary.id(), live.id()],
            ..default()
        };
        assert!(
            !runtime
                .prepare_library(&mut world, Handle::default(), true)
                .unwrap()
        );
        runtime.retired_meshes.retain(|id| *id != temporary.id());
        assert!(
            runtime
                .prepare_library(&mut world, Handle::default(), true)
                .unwrap()
        );
        assert_eq!(runtime.retired_meshes, vec![live.id()]);
    }
    #[test]
    fn canceling_preparation_retires_mesh_view_and_temporary_group() {
        let mut world = World::new();
        world.init_resource::<Assets<Mesh>>();
        let mesh = world.resource_mut::<Assets<Mesh>>().add(Cuboid::default());
        let root = world.spawn_empty().id();
        let child = world.spawn(ChildOf(root)).id();
        let camera = world.spawn_empty().id();
        let mut pool = GroupPool::new();
        let group = pool.allocate().unwrap();
        let mut runtime = StructureRuntime {
            group_pool: Some(pool),
            library_warm: Some(WarmLibrary {
                root,
                camera,
                mesh: mesh.clone(),
                group,
                frames: 0,
            }),
            ..default()
        };
        runtime.retire_library(&mut world);
        assert!(world.get_entity(root).is_err());
        assert!(world.get_entity(child).is_err());
        assert!(world.get_entity(camera).is_err());
        assert!(world.resource::<Assets<Mesh>>().get(mesh.id()).is_none());
        assert_eq!(runtime.group_pool.as_ref().unwrap().free.len(), GROUPS);
        assert_eq!(runtime.retired_meshes, vec![mesh.id()]);
    }
}
