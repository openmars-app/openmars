//! Semantic secondary order for coplanar transparent layers and particles.
//! Back-to-front distance stays primary within the world. Text overlays and
//! credit amounts follow it in their original explicit render-order lanes.
use bevy::{
    core_pipeline::core_3d::Transparent3d,
    prelude::*,
    render::{
        Render, RenderApp, RenderSystems,
        extract_component::{ExtractComponent, ExtractComponentPlugin},
        render_phase::ViewSortedRenderPhases,
        sync_world::{MainEntity, MainEntityHashMap},
    },
};
#[derive(Component, ExtractComponent, Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord)]
#[extract_app(RenderApp)]
pub struct TransparentOrder([i64; 7]);
impl TransparentOrder {
    pub const LABEL: Self = Self([3, 0, 0, 0, 0, 0, 0]);
    pub const CREDIT: Self = Self([4, 0, 0, 0, 0, 0, 0]);
    fn lane(self) -> i64 {
        (self.0[0] - 2).max(0)
    }
    pub const TERRAIN_EDGE: Self = Self([0, 0, 0, 0, 0, 0, 0]);
    pub const BLAST_WAVE: Self = Self([2, 0, 0, 0, 0, 0, 0]);
    pub const OCEAN: Self = Self([0, 1, 0, 0, 0, 0, 0]);
    pub const GRID_VISIBLE: Self = Self([0, 2, 0, 0, 0, 0, 0]);
    pub const WORLD_TREE_GROUND: Self = Self([0, 4, 0, 0, 0, 0, 0]);
    pub const WORLD_TREE_CANOPY: Self = Self([0, 5, 0, 0, 0, 0, 0]);
    pub const GRID_OCCLUDED: Self = Self([0, 3, 0, 0, 0, 0, 0]);
    pub const fn particle(key: [i64; 6]) -> Self {
        Self([1, key[0], key[1], key[2], key[3], key[4], key[5]])
    }
}
fn stable_transparent_ties(
    phases: Option<ResMut<ViewSortedRenderPhases<Transparent3d>>>,
    orders: Query<(&MainEntity, &TransparentOrder)>,
) {
    let Some(mut phases) = phases else { return };
    // Sorted mesh items still carry PLACEHOLDER as their render entity here.
    // Join extracted keys through the original main-world identity instead.
    let orders: MainEntityHashMap<_> = orders.iter().map(|(main, order)| (*main, *order)).collect();
    for phase in phases.0.values_mut() {
        phase.items.sort_by_key(|_, item| {
            let order = orders.get(&item.entity.1).copied();
            (
                order.map_or(0, TransparentOrder::lane),
                bevy::math::FloatOrd(item.distance),
                order,
            )
        });
    }
}

pub struct TransparentOrderPlugin;
impl Plugin for TransparentOrderPlugin {
    fn build(&self, app: &mut App) {
        app.add_plugins(ExtractComponentPlugin::<TransparentOrder>::default());
        if let Some(render) = app.get_sub_app_mut(RenderApp) {
            render.add_systems(
                Render,
                stable_transparent_ties
                    .after(RenderSystems::PhaseSort)
                    .before(RenderSystems::Prepare),
            );
        }
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    use bevy::{
        core_pipeline::core_3d::TransparentSortingInfo3d,
        ecs::system::RunSystemOnce,
        render::{
            render_phase::{DrawFunctionId, PhaseItemExtraIndex, SortedRenderPhase},
            render_resource::CachedRenderPipelineId,
            view::RetainedViewEntity,
        },
    };

    fn sorted(allocations: usize, insertion: [usize; 3], distances: [f32; 3]) -> Vec<usize> {
        sorted_keys(
            allocations,
            insertion,
            distances,
            std::array::from_fn(|i| TransparentOrder::particle([0, 0, 0, 16, 0, i as i64])),
        )
    }
    fn sorted_keys(
        allocations: usize,
        insertion: [usize; 3],
        distances: [f32; 3],
        keys: [TransparentOrder; 3],
    ) -> Vec<usize> {
        let mut main = World::new();
        let identities: Vec<MainEntity> = (0..3).map(|_| main.spawn_empty().id().into()).collect();
        let view = RetainedViewEntity::new(main.spawn_empty().id().into(), None, 0);
        let mut render = World::new();
        for _ in 0..allocations {
            render.spawn_empty();
        }
        // Main and render identities differ. This is the actual extracted
        // component layout while the phase item render field is PLACEHOLDER.
        for i in insertion {
            render.spawn((identities[i], keys[i]));
        }
        let mut phase = SortedRenderPhase::<Transparent3d>::default();
        for i in insertion {
            phase.add_retained(Transparent3d {
                sorting_info: TransparentSortingInfo3d::Sorted {
                    mesh_center: Vec3::ZERO,
                    depth_bias: 0.,
                },
                distance: distances[i],
                pipeline: CachedRenderPipelineId::INVALID,
                entity: (Entity::PLACEHOLDER, identities[i]),
                draw_function: DrawFunctionId(0),
                batch_range: 0..1,
                extra_index: PhaseItemExtraIndex::None,
                indexed: true,
            });
        }
        let mut phases = ViewSortedRenderPhases::<Transparent3d>::default();
        phases.0.insert(view, phase);
        render.insert_resource(phases);
        render.run_system_once(stable_transparent_ties).unwrap();
        render.resource::<ViewSortedRenderPhases<Transparent3d>>().0[&view]
            .items
            .values()
            .map(|item| {
                identities
                    .iter()
                    .position(|id| *id == item.entity.1)
                    .unwrap()
            })
            .collect()
    }

    #[test]
    fn equal_depth_sprites_keep_semantic_order_after_reallocation_and_reverse_extraction() {
        assert_eq!(sorted(5, [0, 1, 2], [4.; 3]), vec![0, 1, 2]);
        assert_eq!(sorted(19, [2, 1, 0], [4.; 3]), vec![0, 1, 2]);
        assert_eq!(sorted(41, [1, 2, 0], [4.; 3]), vec![0, 1, 2]);
    }
    #[test]
    fn semantic_ties_preserve_primary_back_to_front_distance() {
        assert_eq!(sorted(11, [0, 1, 2], [-1., -3., -2.]), vec![1, 2, 0]);
    }
    #[test]
    fn coplanar_terrain_edge_precedes_water_after_reverse_extraction() {
        let keys = [
            TransparentOrder::GRID_VISIBLE,
            TransparentOrder::TERRAIN_EDGE,
            TransparentOrder::OCEAN,
        ];
        assert_eq!(sorted_keys(31, [0, 2, 1], [4.; 3], keys), vec![1, 2, 0]);
        assert_eq!(sorted_keys(7, [1, 2, 0], [4.; 3], keys), vec![1, 2, 0]);
    }
    #[test]
    fn complementary_grid_pass_order_is_independent_of_entity_allocation() {
        let keys = [
            TransparentOrder::GRID_OCCLUDED,
            TransparentOrder::GRID_VISIBLE,
            TransparentOrder::particle([0; 6]),
        ];
        assert_eq!(sorted_keys(17, [2, 0, 1], [4.; 3], keys), vec![1, 0, 2]);
    }
    #[test]
    fn world_precedes_labels_and_credit_digits_regardless_of_camera_distance() {
        let keys = [
            TransparentOrder::CREDIT,
            TransparentOrder::GRID_VISIBLE,
            TransparentOrder::LABEL,
        ];
        assert_eq!(
            sorted_keys(23, [2, 0, 1], [-100., -1., -200.], keys),
            vec![1, 2, 0]
        );
        assert_eq!(
            sorted_keys(41, [0, 1, 2], [-1., -200., -100.], keys),
            vec![1, 2, 0]
        );
    }
}
