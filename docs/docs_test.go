package docs

import "testing"

func TestSwaggerInfoRegistered(t *testing.T) {
	if SwaggerInfo == nil {
		t.Fatalf("swagger info must not be nil")
	}
	if SwaggerInfo.InstanceName() == "" {
		t.Fatalf("swagger instance name should not be empty")
	}
	if SwaggerInfo.SwaggerTemplate == "" {
		t.Fatalf("swagger template should not be empty")
	}
}
