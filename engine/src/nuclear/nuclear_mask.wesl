import openmars_engine::nuclear::nuclear_io::{Vertex,Output,vertex as project,visible};
@vertex fn vertex(input:Vertex)->Output{return project(input);}
@fragment fn fragment(in:Output)->@location(0) vec4<f32>{if !visible(in.position.xy){discard;}return vec4<f32>(0.0,0.0,1.0,1.0);}
