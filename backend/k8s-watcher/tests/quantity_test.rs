use k8s_watcher::quantity::parse_quantity;

#[test]
fn parses_binary_decimal_and_plain_quantities() {
    assert_eq!(parse_quantity("407Mi"), Some(407.0 * 1048576.0));
    assert_eq!(parse_quantity("1Gi"), Some(1073741824.0));
    assert_eq!(parse_quantity("123456Ki"), Some(123456.0 * 1024.0));
    assert_eq!(parse_quantity("500M"), Some(5e8));
    assert_eq!(parse_quantity("100m"), Some(0.1));
    assert_eq!(parse_quantity("2048"), Some(2048.0));
}

#[test]
fn rejects_garbage() {
    assert_eq!(parse_quantity("lots"), None);
}
