// two letter units come first, so "Mi" is never read as "M"
const UNITS: [(&str, f64); 9] = [
    ("Ki", 1024.0),
    ("Mi", 1048576.0),
    ("Gi", 1073741824.0),
    ("Ti", 1099511627776.0),
    ("k", 1e3),
    ("M", 1e6),
    ("G", 1e9),
    ("T", 1e12),
    ("m", 1e-3),
];

/// Parses a kubernetes quantity such as "407Mi" or "1Gi" into a plain number.
pub fn parse_quantity(quantity: &str) -> Option<f64> {
    for (unit, factor) in UNITS {
        if let Some(number) = quantity.strip_suffix(unit) {
            return number.parse::<f64>().ok().map(|n| n * factor);
        }
    }
    quantity.parse().ok()
}
