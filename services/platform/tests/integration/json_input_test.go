package integration

import "testing"

func TestJSONNULRejectedBeforeStorage(t *testing.T) {
	f := setup(t)
	f.request(t, "POST", f.merchantPath, f.token, "nul-store-0001", `{"display_name":"bad\u0000name"}`, 400)
	store := preparedStore(t, f)
	f.request(t, "PUT", f.merchantPath+"/"+store.ID.String()+"/offering", f.token, "nul-menu-0001", `{"base_revision":1,"model":{"id":"ocp.food.menu","version":"1.0.0"},"document":{"nested":["bad\u0000name"]}}`, 400)
}
