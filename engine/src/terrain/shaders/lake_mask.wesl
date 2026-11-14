import openmars_engine::terrain::shaders::field::{terrain_vertex,TerrainVertex,basin_shore};
import bevy_pbr::render::forward_io::Vertex;
@vertex fn vertex(input:Vertex)->TerrainVertex {return terrain_vertex(input,false);}
@fragment fn fragment(in:TerrainVertex)->@location(0) vec4<f32> {
    if basin_shore(in.local,in.layer_birth)>=0.09 {discard;}
    // Pipeline writes GREEN only, preserving independent Mohole/nuclear bits.
    return vec4<f32>(0.0,1.0,0.0,1.0);
}
