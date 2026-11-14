//! Stateless volcano height and flow kernels, in tile-local units.
use super::{
    noise::{simplex, smooth},
    terrain::TerrainError,
};

pub const VOLCANO_FLOW_GRID: usize = 64;

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct VolcanoParams {
    pub seed: f32,
    pub height: f32,
    pub crater_radius: f32,
    pub crater_depth: f32,
}
pub const VOLCANO_DEFAULT: VolcanoParams = VolcanoParams {
    seed: 0.0,
    height: 0.14,
    crater_radius: 0.22,
    crater_depth: 0.08,
};
impl VolcanoParams {
    fn validate(self) -> Result<(), TerrainError> {
        if ![
            self.seed,
            self.height,
            self.crater_radius,
            self.crater_depth,
        ]
        .iter()
        .all(|x| x.is_finite())
        {
            return Err(TerrainError::NonFinite);
        }
        if self.height < 0.0 || self.crater_radius <= 0.0 || self.crater_depth < 0.0 {
            return Err(TerrainError::InvalidVolcano);
        }
        Ok(())
    }
}

fn seed_param(seed: f64, index: f64) -> f64 {
    let n = (seed * 127.1 + index * 311.7).sin() * 43758.5453123;
    ((n % 1.0) + 1.0) % 1.0
}

/// Same intermediate precision and formulas as the existing CPU reference.
/// The [-1,1] domain and seed are explicit, with no mutable RNG state.
pub fn volcano_height(cx: f32, cy: f32, p: &VolcanoParams) -> Result<f32, TerrainError> {
    p.validate()?;
    if !cx.is_finite() || !cy.is_finite() {
        return Err(TerrainError::NonFinite);
    }
    if cx.abs() > 1.0 || cy.abs() > 1.0 {
        return Err(TerrainError::OutsideDomain);
    }
    let h = height(f64::from(cx), f64::from(cy), p) as f32;
    if !h.is_finite() {
        return Err(TerrainError::NonFinite);
    }
    Ok(h)
}

/// Surface normals need the source's central differences at the outer rim too.
/// Keep public sample centers within the existing domain; only the derivative
/// stencil extends .008 beyond it, as in the original vertex shader.
pub fn volcano_surface_sample(
    cx: f32,
    cy: f32,
    p: &VolcanoParams,
) -> Result<[f32; 3], TerrainError> {
    let center = volcano_height(cx, cy, p)?;
    let x = f64::from(cx);
    let y = f64::from(cy);
    let e = 0.008;
    let dx = ((height(x + e, y, p) as f32) - (height(x - e, y, p) as f32)) / 0.016;
    let dy = ((height(x, y + e, p) as f32) - (height(x, y - e, p) as f32)) / 0.016;
    if ![center, dx, dy].into_iter().all(f32::is_finite) {
        return Err(TerrainError::NonFinite);
    }
    Ok([center, dx, dy])
}

/// Upload the same seeded parameters used by CPU heights and flow. Evaluating
/// sin/fract again in a GPU f32 shader would choose a different crater/warp.
pub fn volcano_seed_parameters(p: &VolcanoParams) -> Result<[[f32; 4]; 4], TerrainError> {
    p.validate()?;
    Ok(std::array::from_fn(|row| {
        std::array::from_fn(|col| seed_param(f64::from(p.seed), (row * 4 + col) as f64) as f32)
    }))
}

fn height(cx: f64, cy: f64, p: &VolcanoParams) -> f64 {
    let seed = f64::from(p.seed);
    let sp = |index| seed_param(seed, index);
    let cone_height = f64::from(p.height) * (0.85 + sp(0.0) * 0.3);
    let crater_rad = f64::from(p.crater_radius) * (0.85 + sp(1.0) * 0.3);
    let crater_dep = f64::from(p.crater_depth) * (0.8 + sp(2.0) * 0.4);
    let rim_height = 0.012 + sp(3.0) * 0.012;
    let rim_width = 0.06 + sp(4.0) * 0.04;
    let crater_x = (sp(5.0) - 0.5) * 0.05;
    let crater_y = (sp(6.0) - 0.5) * 0.05;
    let ellipse_x = 0.9 + sp(7.0) * 0.2;
    let ellipse_y = 0.9 + sp(8.0) * 0.2;
    let cone_x = (sp(9.0) - 0.5) * 0.08;
    let cone_y = (sp(10.0) - 0.5) * 0.08;
    let frequency = 5.0 + sp(11.0) * 4.0;
    let sx = sp(12.0) * 100.0;
    let sy = sp(13.0) * 100.0;
    let n1 = simplex(cx * 1.8 + sx, cy * 1.8 + sy);
    let n2 = simplex(cx * 1.8 + sx + 50.0, cy * 1.8 + sy + 50.0);
    let mask = smooth(0.12, 0.35, (cx * cx + cy * cy).sqrt());
    let wx = cx + cone_x + n1 * (0.12 * mask);
    let wy = cy + cone_y + n2 * (0.12 * mask);
    let s1 = simplex(wx * 0.7 + sx * 0.05, wy * 0.7 + sy * 0.05);
    let s2 = simplex(wx * 0.35 + sx * 0.05 + 19.1, wy * 0.35 + sy * 0.05 + 19.1);
    let shape = s1 * 0.7 + s2 * 0.3;
    let dist = (wx * wx + wy * wy).sqrt() * (1.0 + shape * 0.18);
    let mut h = (1.0 - dist).max(0.0).powf(1.4) * cone_height;
    let apron = (-dist * dist * 5.0).exp() * cone_height * 0.12;
    let apron_noise = simplex(cx * 2.0 + sx + 200.0, cy * 2.0 + sy + 200.0) * 0.5 + 0.5;
    h += apron * (1.0 + 0.4 * (apron_noise - 0.5));
    let cpx = (wx - crater_x) * ellipse_x;
    let cpy = (wy - crater_y) * ellipse_y;
    let cd = (cpx * cpx + cpy * cpy).sqrt();
    let rim_t = (cd - crater_rad) / rim_width;
    let rim = (-rim_t * rim_t).exp() * rim_height;
    let rim_noise = simplex(cpx * 3.0 + sx * 0.05, cpy * 3.0 + sy * 0.05) * 0.4 + 0.8;
    h += rim * (rim_noise * smooth(0.8, 0.4, dist));
    let bowl = 1.0 - smooth(0.0, 1.0, cd / crater_rad.max(0.001));
    h -= bowl * bowl * crater_dep;
    let g1 = simplex(wx * frequency + sx * 0.05, wy * frequency + sy * 0.05);
    let g2 = simplex(
        wx * (frequency * 2.0) + sx * 0.05 + 31.7,
        wy * (frequency * 2.0) + sy * 0.05 + 31.7,
    );
    let ridge = (1.0 - (g1 * 0.8 + g2 * 0.2).abs()).powi(2);
    h -= ridge * (smooth(0.18, 0.32, dist) * smooth(0.88, 0.58, dist)) * 0.03;
    let n1 = simplex(cx * 8.0 + sx + 3.7, cy * 8.0 + sy + 3.7);
    let n2 = simplex(cx * 16.0 + sx + 11.3, cy * 16.0 + sy + 11.3);
    h + (n1 * 0.65 + n2 * 0.35) * 0.004 * (1.0 - dist * 0.7).max(0.0)
}

/// Normalized 64x64 f32 flow, row-major, row increases tile-local cy. Heights,
/// accumulations and each blur write round to f32 as in Float32Array. Equal
/// heights use stable row-major order; equal slopes retain the first neighbor.
pub fn volcano_flow_map(p: &VolcanoParams) -> Result<Vec<f32>, TerrainError> {
    let mut work = VolcanoFlowWork::new(*p)?;
    while !work.advance()? {}
    work.take_ready().ok_or(TerrainError::InvalidVolcano)
}

/// One bounded flow-map owner: 256 height samples or 512 cheap routing/blur
/// cells per turn. Dropping this value cancels preparation without publication.
pub struct VolcanoFlowWork {
    params: VolcanoParams,
    heights: Vec<f32>,
    flow: Vec<f32>,
    next: Vec<f32>,
    order: Vec<usize>,
    stage: u8,
    cursor: usize,
    taken: bool,
}
impl VolcanoFlowWork {
    pub const PEAK_BYTES: usize = 4096 * (3 * 4 + std::mem::size_of::<usize>());
    pub fn new(params: VolcanoParams) -> Result<Self, TerrainError> {
        params.validate()?;
        let n = VOLCANO_FLOW_GRID * VOLCANO_FLOW_GRID;
        Ok(Self {
            params,
            heights: vec![0.; n],
            flow: vec![0.; n],
            next: vec![0.; n],
            order: (0..n).collect(),
            stage: 0,
            cursor: 0,
            taken: false,
        })
    }
    pub fn advance(&mut self) -> Result<bool, TerrainError> {
        if self.taken {
            return Err(TerrainError::InvalidVolcano);
        }
        let size = VOLCANO_FLOW_GRID;
        let n = size * size;
        match self.stage {
            0 => {
                let crater = f64::from(self.params.crater_radius)
                    * (0.85 + seed_param(f64::from(self.params.seed), 1.) * 0.3);
                for i in self.cursor..(self.cursor + 256).min(n) {
                    let cx = ((i % size) as f64 + 0.5) / size as f64 * 2. - 1.;
                    let cy = ((i / size) as f64 + 0.5) / size as f64 * 2. - 1.;
                    self.heights[i] = height(cx, cy, &self.params) as f32;
                    if !self.heights[i].is_finite() {
                        return Err(TerrainError::NonFinite);
                    }
                    if ((cx * cx + cy * cy).sqrt() - crater).abs() < 0.08 {
                        self.flow[i] = 1.;
                    }
                    self.cursor = i + 1;
                }
            }
            1 => {
                self.order
                    .sort_by(|a, b| self.heights[*b].total_cmp(&self.heights[*a]));
                self.cursor = n;
            }
            2 => {
                for at in self.cursor..(self.cursor + 512).min(n) {
                    let i = self.order[at];
                    self.cursor = at + 1;
                    if self.flow[i] <= 0. {
                        continue;
                    }
                    let mut best = None;
                    let mut slope = 0.;
                    for (dx, dy) in [
                        (-1, -1),
                        (0, -1),
                        (1, -1),
                        (-1, 0),
                        (1, 0),
                        (-1, 1),
                        (0, 1),
                        (1, 1),
                    ] {
                        let x = (i % size) as i32 + dx;
                        let y = (i / size) as i32 + dy;
                        if x < 0 || y < 0 || x >= size as i32 || y >= size as i32 {
                            continue;
                        }
                        let j = y as usize * size + x as usize;
                        let distance = if dx != 0 && dy != 0 { 1.414 } else { 1. };
                        let s =
                            (f64::from(self.heights[i]) - f64::from(self.heights[j])) / distance;
                        if s > slope {
                            slope = s;
                            best = Some(j);
                        }
                    }
                    if let Some(j) = best {
                        self.flow[j] += self.flow[i];
                    }
                }
            }
            3 | 6 => {
                normalize(&mut self.flow);
                self.cursor = n;
            }
            4 | 5 => {
                for i in self.cursor..(self.cursor + 512).min(n) {
                    let x = (i % size) as i32;
                    let y = (i / size) as i32;
                    let mut sum = 0.;
                    let mut weight = 0.;
                    for dy in -1_i32..=1 {
                        for dx in -1_i32..=1 {
                            let nx = x + dx;
                            let ny = y + dy;
                            if nx < 0 || ny < 0 || nx >= size as i32 || ny >= size as i32 {
                                continue;
                            }
                            let w = if dx == 0 && dy == 0 {
                                4.
                            } else if dx.abs() + dy.abs() == 1 {
                                2.
                            } else {
                                1.
                            };
                            sum += f64::from(self.flow[ny as usize * size + nx as usize]) * w;
                            weight += w;
                        }
                    }
                    self.next[i] = (sum / weight) as f32;
                    self.cursor = i + 1;
                }
                if self.cursor == n {
                    std::mem::swap(&mut self.next, &mut self.flow);
                }
            }
            _ => return Ok(true),
        }
        if self.cursor == n {
            self.stage += 1;
            self.cursor = 0;
        }
        Ok(self.stage == 7)
    }
    pub fn take_ready(&mut self) -> Option<Vec<f32>> {
        if self.stage != 7 || self.taken {
            return None;
        }
        self.taken = true;
        Some(std::mem::take(&mut self.flow))
    }
}
fn normalize(values: &mut [f32]) {
    let max = values.iter().copied().fold(0.0_f32, f32::max);
    if max > 0.0 {
        let inverse = 1.0 / f64::from(max);
        for v in values {
            *v = (f64::from(*v) * inverse) as f32;
        }
    }
}
