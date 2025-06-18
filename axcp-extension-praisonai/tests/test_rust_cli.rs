use axcp_praison_bridge::{wrap, unwrap};
use axcp_praison_bridge::pb::AxcpEnvelope;

#[test]
fn test_wrap_unwrap_trace_id() {
    let env = AxcpEnvelope {
        trace_id: "test123".to_string(),
        ..Default::default()
    };
    let data = wrap(env.clone());
    let parsed = unwrap(&data);
    assert_eq!(parsed.trace_id, "test123");
}
