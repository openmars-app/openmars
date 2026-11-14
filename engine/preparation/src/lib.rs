//! The worker contains pure scene math only; it cannot allocate GPU resources.
use wasm_bindgen::prelude::*;
#[wasm_bindgen]
#[derive(Default)]
pub struct TerrainPreparer(openmars_scene::landscape::TerrainWorker);
#[wasm_bindgen]
impl TerrainPreparer {
    #[wasm_bindgen(constructor)]
    pub fn new() -> Self {
        Self::default()
    }
    pub fn metadata(&self) -> String {
        self.0.metadata()
    }
    pub fn prepare(&mut self, input: &[u8]) -> Result<Vec<u8>, JsValue> {
        self.0.run(input).map_err(|e| JsValue::from_str(&e))
    }
}
