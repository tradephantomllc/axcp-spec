#!/usr/bin/env python3
from copy import deepcopy

doc = {"battery": 80, "location": "Lab"}
patch = [
    {"op": "REPLACE", "path": "/battery", "data": 95, "ts": 100},
    {"op": "MERGE", "path": "/metrics", "data": {"steps": 1200}, "ts": 101}
]

def apply(doc, ops):
    out = deepcopy(doc)
    for op in ops:
        if op["op"] == "REPLACE":
            key = op["path"].lstrip("/")
            out[key] = op["data"]
        elif op["op"] == "MERGE":
            key = op["path"].lstrip("/")
            out.setdefault(key, {}).update(op["data"])
    return out

def test():
    res = apply(doc, patch)
    assert res["battery"] == 95
    assert res["metrics"]["steps"] == 1200
    print("delta merge OK")

if __name__ == "__main__":
    test()
