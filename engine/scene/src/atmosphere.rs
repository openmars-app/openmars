//! Validated atmosphere profiles and projected visibility, shared by GPU backends.
pub const MAX_BODIES: usize = 16;
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Profile {
    pub color: [f64; 3],
    pub shadow: [f64; 3],
    pub thickness: f64,
    pub intensity: f64,
    pub halo: f64,
}

// Match Three's actual transfer functions, including its legacy exponent.
pub fn srgb_to_linear(c: f64) -> f64 {
    if c < 0.04045 {
        c * 0.0773993808
    } else {
        (c * 0.9478672986 + 0.0521327014).powf(2.4)
    }
}
pub fn linear_to_srgb(c: f64) -> f64 {
    if c < 0.0031308 {
        c * 12.92
    } else {
        1.055 * c.powf(0.41666) - 0.055
    }
}
fn rgb(hex: u32) -> [f64; 3] {
    [hex >> 16, hex >> 8, hex].map(|v| srgb_to_linear(f64::from(v & 255) / 255.))
}
fn mix(a: f64, b: f64, t: f64) -> f64 {
    a + (b - a) * t
}
fn smooth(a: f64, b: f64, x: f64) -> f64 {
    let t = ((x - a) / (b - a)).clamp(0., 1.);
    t * t * (3. - 2. * t)
}
impl Profile {
    pub fn validate(&self) -> Result<(), &'static str> {
        if self
            .color
            .into_iter()
            .chain(self.shadow)
            .any(|c| !c.is_finite() || !(0. ..=1.).contains(&c))
            || !self.thickness.is_finite()
            || !(0.0001..=1.).contains(&self.thickness)
            || !self.intensity.is_finite()
            || !(0. ..=16.).contains(&self.intensity)
            || !self.halo.is_finite()
            || !(0. ..=16.).contains(&self.halo)
        {
            return Err("Invalid atmosphere profile");
        }
        Ok(())
    }
}

pub fn mars_profile(sky: f64, warm: f64) -> Result<Profile, &'static str> {
    if !sky.is_finite()
        || !warm.is_finite()
        || !(0. ..=1.).contains(&sky)
        || !(0. ..=1.).contains(&warm)
    {
        return Err("Invalid atmosphere climate");
    }
    let presets = [
        (0xc3cddb, 0x56708f, 0.045, 0.85, 0.9),
        (0xf0b9a2, 0x5b7d9c, 0.055, 1.1, 1.),
        (0xec9468, 0x7a4636, 0.07, 1.25, 1.1),
    ];
    let position = sky * 2.;
    let index = (position.floor() as usize).min(1);
    let t = position - index as f64;
    let (a, b) = (presets[index], presets[index + 1]);
    // MarsAtmosphere lerps THREE.Color, writes getHexString, then parses it
    // again for shader uniforms. Preserve that authored eight-bit round trip.
    let color = |a, b| {
        let (a, b) = (rgb(a), rgb(b));
        std::array::from_fn(|i| {
            srgb_to_linear((linear_to_srgb(mix(a[i], b[i], t)).clamp(0., 1.) * 255.).round() / 255.)
        })
    };
    Ok(Profile {
        color: color(a.0, b.0),
        shadow: color(a.1, b.1),
        thickness: mix(a.2, b.2, t),
        intensity: mix(a.3, b.3, t) * (1. + warm * 0.6),
        halo: mix(a.4, b.4, t),
    })
}

/// CSS pixels, exactly as the old AtmosphereRenderer's projected-radius gate.
pub fn visibility(
    radius: f64,
    view_z: f64,
    projection_y: f64,
    height: f64,
) -> Result<f64, &'static str> {
    if ![radius, view_z, projection_y, height]
        .into_iter()
        .all(f64::is_finite)
        || radius <= 0.
        || projection_y <= 0.
        || height <= 0.
    {
        return Err("Invalid atmosphere view");
    }
    Ok(smooth(
        3.,
        12.,
        radius * projection_y * height / (2. * (-view_z).max(radius)),
    ))
}

#[derive(Clone, Copy, Debug)]
pub struct Body {
    pub center: [f64; 3],
    pub radius: f64,
    pub profile: Profile,
    pub follows_mesh: bool,
}
impl Body {
    pub fn validate(&self) -> Result<(), &'static str> {
        self.profile.validate()?;
        if self
            .center
            .into_iter()
            .any(|v| !v.is_finite() || v.abs() > 1e8)
            || !self.radius.is_finite()
            || !(1e-6..=1e8).contains(&self.radius)
        {
            return Err("Invalid atmosphere body");
        }
        Ok(())
    }
}
