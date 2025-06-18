use axcp_praison_bridge::{wrap, unwrap};
use axcp_praison_bridge::pb::AxcpEnvelope;

#[test]
fn trace_id_roundtrip() {
    let env = AxcpEnvelope {
        trace_id: "test123".to_string(),
        ..Default::default()
    };
    let bytes = wrap(env.clone());
    let parsed = unwrap(&bytes);
    assert_eq!(parsed.trace_id, env.trace_id);
}
