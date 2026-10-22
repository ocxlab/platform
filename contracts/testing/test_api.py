"""Bounded Schemathesis exploration. Only the private Compose test API is reachable."""
import json
import sys
import uuid
from collections import Counter
from pathlib import Path

import schemathesis
from hypothesis import HealthCheck, given, settings

BASE = "http://browser-api:8080"
SCHEMA = Path("/workspace/contracts/build/provider.openapi.json")
REPORT = Path("/reports/schemathesis.json")
COUNTS = Counter()
STATUSES = Counter()
SKIPPED = []
CHECKS = [schemathesis.checks.not_a_server_error,
          schemathesis.checks.status_code_conformance,
          schemathesis.checks.content_type_conformance,
          schemathesis.checks.response_headers_conformance,
          schemathesis.checks.response_schema_conformance]


def main():
    fixture = json.load(sys.stdin)
    schema = schemathesis.openapi.from_path(SCHEMA)
    # Explicit allowlist: no SSE, media transfers, external callbacks or payments.
    ops = {}
    for result in schema.get_all_operations():
        operation = result.ok()
        ops[operation.definition.raw["operationId"]] = operation
    merchant = fixture["merchant_id"]
    token = fixture["merchant_token"]
    header = {"Authorization": f"Bearer {token}"}
    foreign = fixture["extras_merchant_id"]

    def call(name, *, paths=None, body=None, query=None, auth=header, expected=None):
        payload = {"body": body, "media_type": "application/json"} if body is not None else {}
        case = ops[name].Case(path_parameters=paths,
                              query=query, headers={**auth, "Idempotency-Key": str(uuid.uuid4())},
                              **payload)
        response = case.call_and_validate(base_url=BASE, checks=CHECKS, timeout=5)
        COUNTS[name] += 1
        STATUSES[str(response.status_code)] += 1
        if expected is not None:
            assert response.status_code == expected, (name, response.status_code)
        return response.json()

    # Seed a meaningful state through the real API, validating every response.
    call("CreateStore", paths={"merchantId": merchant},
         body={"display_name": "invalid\x00name"}, expected=400)
    call("ListPublicStores", query={"q": "invalid\x00query"}, auth={}, expected=400)
    created = call("CreateStore", paths={"merchantId": merchant},
                   body={"display_name": "Contract testing kitchen"}, expected=201)
    paths = {"merchantId": merchant, "storeId": created["id"]}
    call("WriteOffering", paths=paths, body={"base_revision": 0,
         "model": {"id": "ocp.food.menu", "version": "1.0.0"},
         "document": {"nested": ["invalid\x00value"]}}, expected=400)
    menu = json.loads(fixture["demo_menu"])
    call("WriteOffering", paths=paths, body={"base_revision": 0,
         "model": {"id": "ocp.food.menu", "version": "1.0.0"}, "document": menu}, expected=201)
    call("ChangePublication", paths=paths, body={"expected_generation": 0, "revision": 1}, expected=201)
    guest = call("CreateGuestSession", paths=paths, body={}, auth={}, expected=201)
    guest_auth = {"Authorization": f"Bearer {guest['access_token']}"}
    first = menu["items"][0]
    selection = {"publication_generation": 1, "lines": [{"item_id": first["id"],
        "variant_id": first["variants"][0]["id"], "quantity": 1,
        "options": [{"group_id": group["id"], "option_id": option}
                    for group in first["option_groups"]
                    for option in (group.get("default_option_ids") or
                                   [o["id"] for o in group["options"][:group["min_selected"]]])]}]}
    quote = call("CreateGuestFoodQuote", paths=paths, body=selection, auth=guest_auth, expected=201)
    placed = call("PlaceFoodOrder", paths=paths, body={"quote_id": quote["id"], "fulfilment": "pickup"}, auth=guest_auth, expected=201)
    call("GetGuestFoodOrder", paths={**paths, "orderId": placed["id"]}, auth=guest_auth, expected=200)
    call("GetMerchantFoodOrder", paths={**paths, "orderId": placed["id"]}, expected=200)
    # Explicit authorization controls, not merely accepting 401 responses to random IDs.
    call("GetStore", paths=paths, auth={}, expected=401)
    call("GetStore", paths={**paths, "merchantId": foreign}, expected=403)

    targets = [
        ("ListPublicStores", {}, {}),
        ("ListStores", {"merchantId": merchant}, header),
        ("GetStore", paths, header),
        ("GetOffering", paths, header),
        ("GetPublication", paths, header),
        ("GetFoodOrdering", paths, header),
        ("GetPublicOffering", paths, {}),
        ("ListMerchantFoodOrders", paths, header),
        ("CreateStore", {"merchantId": merchant}, header),
        ("WriteOffering", paths, header),
        ("CreateGuestFoodQuote", paths, guest_auth),
    ]
    # Both shape-valid and deliberately invalid inputs; not a business-validity oracle.
    for name, path_parameters, auth in targets:
        for mode in (schemathesis.GenerationMode.POSITIVE, schemathesis.GenerationMode.NEGATIVE):
            if mode == schemathesis.GenerationMode.NEGATIVE and name in {
                "GetStore", "GetOffering", "GetPublication", "GetFoodOrdering", "GetPublicOffering"
            }:
                # All declared inputs are fixed path parameters. Auth negatives are explicit above.
                SKIPPED.append(f"{name}:negative (no unfixed schema input)")
                continue
            strategy = ops[name].as_strategy(generation_mode=mode, path_parameters=path_parameters,
                                            headers={**auth, "Idempotency-Key": str(uuid.uuid4())})

            @settings(max_examples=20, deadline=None, database=None, derandomize=True,
                      suppress_health_check=[HealthCheck.too_slow, HealthCheck.filter_too_much])
            @given(case=strategy)
            def exercise(case):
                # Each mutation is distinct; matching-key replay has separate deterministic tests.
                case.headers["Idempotency-Key"] = str(uuid.uuid4())
                response = case.call_and_validate(base_url=BASE, checks=CHECKS, timeout=5)
                COUNTS[f"{name}:{mode.value}"] += 1
                STATUSES[str(response.status_code)] += 1

            print(f"Exploring {name} ({mode.value})", flush=True)
            exercise()


if __name__ == "__main__":
    passed = False
    try:
        main()
        passed = True
    finally:
        REPORT.parent.mkdir(parents=True, exist_ok=True)
        REPORT.write_text(json.dumps({"passed": passed, "cases": dict(COUNTS), "skipped": SKIPPED,
                                      "status_codes": dict(STATUSES)}, indent=2) + "\n")
        print(f"Validated {sum(COUNTS.values())} responses; passed={passed}")
