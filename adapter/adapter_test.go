package adapter

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zhangpanda/gomcp"
	"github.com/zhangpanda/gomcp/mcptest"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func init() { gin.SetMode(gin.TestMode) }

func testGinRouter() *gin.Engine {
	r := gin.New()
	r.GET("/api/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, []map[string]string{{"name": "alice"}})
	})
	r.GET("/api/users/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	})
	r.POST("/api/users", func(c *gin.Context) {
		c.JSON(http.StatusCreated, map[string]string{"created": "true"})
	})
	r.DELETE("/api/admin/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]string{"deleted": c.Param("id")})
	})
	return r
}

func TestImportGin_AllRoutes(t *testing.T) {
	s := gomcp.New("test", "1.0")
	ImportGin(s, testGinRouter(), ImportOptions{})

	c := mcptest.NewClient(t, s)
	tools := c.ListTools()
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d: %v", len(tools), tools)
	}
}

func TestImportGin_IncludeFilter(t *testing.T) {
	s := gomcp.New("test", "1.0")
	ImportGin(s, testGinRouter(), ImportOptions{IncludePaths: []string{"/api/users"}})

	c := mcptest.NewClient(t, s)
	tools := c.ListTools()
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools (users only), got %d: %v", len(tools), tools)
	}
}

func TestImportGin_ExcludeFilter(t *testing.T) {
	s := gomcp.New("test", "1.0")
	ImportGin(s, testGinRouter(), ImportOptions{ExcludePaths: []string{"/api/admin"}})

	c := mcptest.NewClient(t, s)
	tools := c.ListTools()
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools (admin excluded), got %d: %v", len(tools), tools)
	}
}

func TestImportGin_CustomNaming(t *testing.T) {
	s := gomcp.New("test", "1.0")
	ImportGin(s, testGinRouter(), ImportOptions{
		NamingFunc: func(method, path string) string { return method + "_custom" },
	})

	c := mcptest.NewClient(t, s)
	tools := c.ListTools()
	for _, name := range tools {
		if len(name) < 4 {
			t.Errorf("unexpected tool name: %s", name)
		}
	}
}

func TestImportGin_CallRoute(t *testing.T) {
	s := gomcp.New("test", "1.0")
	ImportGin(s, testGinRouter(), ImportOptions{})

	c := mcptest.NewClient(t, s)
	r := c.CallTool("get_api_users_by_id", map[string]any{"id": "42"})
	r.AssertNoError(t)
	r.AssertContains(t, "42")
}

func TestImportGin_PostRoute(t *testing.T) {
	s := gomcp.New("test", "1.0")
	ImportGin(s, testGinRouter(), ImportOptions{})

	c := mcptest.NewClient(t, s)
	r := c.CallTool("post_api_users", map[string]any{"body": `{"name":"bob"}`})
	r.AssertNoError(t)
	r.AssertContains(t, "created")
}

func TestImportGin_QueryParams(t *testing.T) {
	r := gin.New()
	r.GET("/api/search", func(c *gin.Context) {
		q := c.Query("q")
		page := c.DefaultQuery("page", "1")
		c.JSON(200, map[string]string{"q": q, "page": page})
	})

	s := gomcp.New("test", "1.0")
	ImportGin(s, r, ImportOptions{})

	c := mcptest.NewClient(t, s)
	result := c.CallTool("get_api_search", map[string]any{"query": "q=hello&page=3"})
	result.AssertNoError(t)
	result.AssertContains(t, "hello")
	result.AssertContains(t, "3")
}

// --- OpenAPI tests ---

func TestImportOpenAPI(t *testing.T) {
	// Create a temp OpenAPI spec
	spec := `openapi: "3.0.0"
info:
  title: Test
  version: "1.0"
paths:
  /items:
    get:
      operationId: listItems
      summary: List items
      tags: [items]
      parameters:
        - name: limit
          in: query
          schema:
            type: integer
  /items/{id}:
    get:
      operationId: getItem
      summary: Get item
      tags: [items]
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
`
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.yaml")
	os.WriteFile(specFile, []byte(spec), 0o644)

	s := gomcp.New("test", "1.0")
	err := ImportOpenAPI(s, specFile, OpenAPIOptions{ServerURL: "http://localhost"})
	if err != nil {
		t.Fatalf("ImportOpenAPI error: %v", err)
	}

	c := mcptest.NewClient(t, s)
	tools := c.ListTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %v", len(tools), tools)
	}
}

func TestImportOpenAPI_TagFilter(t *testing.T) {
	spec := `openapi: "3.0.0"
info:
  title: Test
  version: "1.0"
paths:
  /a:
    get:
      operationId: opA
      tags: [alpha]
      parameters: []
  /b:
    get:
      operationId: opB
      tags: [beta]
      parameters: []
`
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.yaml")
	os.WriteFile(specFile, []byte(spec), 0o644)

	s := gomcp.New("test", "1.0")
	ImportOpenAPI(s, specFile, OpenAPIOptions{TagFilter: []string{"alpha"}})

	c := mcptest.NewClient(t, s)
	tools := c.ListTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool (alpha only), got %d: %v", len(tools), tools)
	}
}

func TestImportOpenAPI_FileNotFound(t *testing.T) {
	s := gomcp.New("test", "1.0")
	err := ImportOpenAPI(s, "/nonexistent.yaml", OpenAPIOptions{})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestImportOpenAPI_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	specFile := filepath.Join(dir, "bad.yaml")
	os.WriteFile(specFile, []byte("not: [valid: yaml: {"), 0o644)

	s := gomcp.New("test", "1.0")
	err := ImportOpenAPI(s, specFile, OpenAPIOptions{})
	_ = err
}

func TestImportOpenAPI_RefResolution(t *testing.T) {
	spec := `openapi: "3.0.0"
info:
  title: Test
  version: "1.0"
components:
  schemas:
    Pet:
      type: object
      properties:
        name:
          type: string
          description: Pet name
        species:
          type: string
          enum: [dog, cat, bird]
      required: [name]
  parameters:
    LimitParam:
      name: limit
      in: query
      description: Max results
      schema:
        type: integer
paths:
  /pets:
    get:
      operationId: listPets
      summary: List pets
      parameters:
        - $ref: '#/components/parameters/LimitParam'
    post:
      operationId: createPet
      summary: Create a pet
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Pet'
`
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.yaml")
	os.WriteFile(specFile, []byte(spec), 0o644)

	s := gomcp.New("test", "1.0")
	err := ImportOpenAPI(s, specFile, OpenAPIOptions{ServerURL: "http://localhost"})
	if err != nil {
		t.Fatalf("ImportOpenAPI error: %v", err)
	}

	c := mcptest.NewClient(t, s)
	tools := c.ListTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %v", len(tools), tools)
	}

	// Verify the schema via tools/list raw call
	raw := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	resp := s.HandleRaw(context.Background(), raw)

	respStr := string(resp)
	// listPets should have "limit" param from $ref
	if !strings.Contains(respStr, "limit") {
		t.Error("expected 'limit' param from $ref resolution")
	}
	// createPet should have "name" and "species" from requestBody $ref
	if !strings.Contains(respStr, `"name"`) {
		t.Error("expected 'name' param from requestBody $ref")
	}
	if !strings.Contains(respStr, `"species"`) {
		t.Error("expected 'species' param from requestBody $ref")
	}
}

func TestImportOpenAPI_RequestBody(t *testing.T) {
	spec := `openapi: "3.0.0"
info:
  title: Test
  version: "1.0"
paths:
  /users:
    post:
      operationId: createUser
      summary: Create user
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                username:
                  type: string
                  description: Username
                email:
                  type: string
              required: [username]
`
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.yaml")
	os.WriteFile(specFile, []byte(spec), 0o644)

	s := gomcp.New("test", "1.0")
	err := ImportOpenAPI(s, specFile, OpenAPIOptions{ServerURL: "http://localhost"})
	if err != nil {
		t.Fatalf("ImportOpenAPI error: %v", err)
	}

	c := mcptest.NewClient(t, s)
	tools := c.ListTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d: %v", len(tools), tools)
	}

	// Verify schema has username and email as params (not just "body")
	raw := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	resp := s.HandleRaw(context.Background(), raw)
	respStr := string(resp)

	if !strings.Contains(respStr, "username") {
		t.Error("expected 'username' param from requestBody")
	}
	if !strings.Contains(respStr, "email") {
		t.Error("expected 'email' param from requestBody")
	}
	// should NOT have generic "body" param since we extracted properties
	if strings.Contains(respStr, `"body"`) {
		t.Error("should not have generic 'body' param when requestBody has properties")
	}
}

// --- Additional coverage tests ---

func TestImportGin_ErrorRoute(t *testing.T) {
	r := gin.New()
	r.GET("/api/error", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "boom"})
	})

	s := gomcp.New("test", "1.0")
	ImportGin(s, r, ImportOptions{})

	c := mcptest.NewClient(t, s)
	result := c.CallTool("get_api_error", map[string]any{})
	result.AssertIsError(t)
	result.AssertContains(t, "500")
}

func TestImportGin_PostWithBody(t *testing.T) {
	r := gin.New()
	r.POST("/api/data", func(c *gin.Context) {
		var body map[string]any
		c.BindJSON(&body)
		c.JSON(200, body)
	})

	s := gomcp.New("test", "1.0")
	ImportGin(s, r, ImportOptions{})

	c := mcptest.NewClient(t, s)
	result := c.CallTool("post_api_data", map[string]any{"body": `{"key":"value"}`})
	result.AssertNoError(t)
	result.AssertContains(t, "value")
}

func TestImportGin_DeleteRoute(t *testing.T) {
	s := gomcp.New("test", "1.0")
	ImportGin(s, testGinRouter(), ImportOptions{})

	c := mcptest.NewClient(t, s)
	result := c.CallTool("delete_api_admin_by_id", map[string]any{"id": "99"})
	result.AssertNoError(t)
	result.AssertContains(t, "99")
}

func TestGinToolName_Default(t *testing.T) {
	name := ginToolName("GET", "/api/v1/users/:id", nil)
	if name != "get_api_v1_users_by_id" {
		t.Errorf("expected get_api_v1_users_by_id, got %s", name)
	}
}

func TestGinToolName_WildcardParam(t *testing.T) {
	name := ginToolName("GET", "/files/*path", nil)
	if name != "get_files_by_path" {
		t.Errorf("expected get_files_by_path, got %s", name)
	}
}

func TestShouldInclude_NoFilters(t *testing.T) {
	if !shouldInclude("/any/path", "GET", ImportOptions{}) {
		t.Error("should include when no filters")
	}
}

func TestShouldInclude_ExcludeOnly(t *testing.T) {
	opts := ImportOptions{ExcludePaths: []string{"/admin"}}
	if shouldInclude("/admin/users", "GET", opts) {
		t.Error("should exclude /admin paths")
	}
	if !shouldInclude("/api/users", "GET", opts) {
		t.Error("should include non-admin paths")
	}
}

func TestShouldInclude_IncludeAndExclude(t *testing.T) {
	opts := ImportOptions{
		IncludePaths: []string{"/api/"},
		ExcludePaths: []string{"/api/internal"},
	}
	if shouldInclude("/api/internal/debug", "GET", opts) {
		t.Error("exclude should take priority")
	}
	if !shouldInclude("/api/users", "GET", opts) {
		t.Error("should include /api/users")
	}
	if shouldInclude("/other", "GET", opts) {
		t.Error("should not include paths outside include list")
	}
}

// --- OpenAPI additional tests ---

func TestImportOpenAPI_NoOperationID(t *testing.T) {
	spec := `openapi: "3.0.0"
info:
  title: Test
  version: "1.0"
paths:
  /items/{id}:
    get:
      summary: Get item
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
    delete:
      summary: Delete item
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
`
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.yaml")
	os.WriteFile(specFile, []byte(spec), 0o644)

	s := gomcp.New("test", "1.0")
	ImportOpenAPI(s, specFile, OpenAPIOptions{})

	c := mcptest.NewClient(t, s)
	tools := c.ListTools()
	// should fallback to method_path naming
	found := map[string]bool{}
	for _, name := range tools {
		found[name] = true
	}
	if !found["get_items_by_id"] {
		t.Errorf("expected get_items_by_id, got %v", tools)
	}
	if !found["delete_items_by_id"] {
		t.Errorf("expected delete_items_by_id, got %v", tools)
	}
}

func TestImportOpenAPI_CustomNaming(t *testing.T) {
	spec := `openapi: "3.0.0"
info:
  title: Test
  version: "1.0"
paths:
  /pets:
    get:
      operationId: listPets
      summary: List
      parameters: []
`
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.yaml")
	os.WriteFile(specFile, []byte(spec), 0o644)

	s := gomcp.New("test", "1.0")
	ImportOpenAPI(s, specFile, OpenAPIOptions{
		NamingFunc: func(opID, method, path string) string { return "custom_" + opID },
	})

	c := mcptest.NewClient(t, s)
	tools := c.ListTools()
	if len(tools) != 1 || tools[0] != "custom_listPets" {
		t.Errorf("expected custom_listPets, got %v", tools)
	}
}

func TestImportOpenAPI_AllMethods(t *testing.T) {
	spec := `openapi: "3.0.0"
info:
  title: Test
  version: "1.0"
paths:
  /resource:
    get:
      operationId: getRes
      summary: Get
      parameters: []
    post:
      operationId: createRes
      summary: Create
    put:
      operationId: updateRes
      summary: Update
    delete:
      operationId: deleteRes
      summary: Delete
    patch:
      operationId: patchRes
      summary: Patch
`
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.yaml")
	os.WriteFile(specFile, []byte(spec), 0o644)

	s := gomcp.New("test", "1.0")
	ImportOpenAPI(s, specFile, OpenAPIOptions{})

	c := mcptest.NewClient(t, s)
	tools := c.ListTools()
	if len(tools) != 5 {
		t.Fatalf("expected 5 tools (all methods), got %d: %v", len(tools), tools)
	}
}

func TestImportOpenAPI_ServerURLFromSpec(t *testing.T) {
	spec := `openapi: "3.0.0"
info:
  title: Test
  version: "1.0"
servers:
  - url: https://api.example.com
paths:
  /ping:
    get:
      operationId: ping
      summary: Ping
      parameters: []
`
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.yaml")
	os.WriteFile(specFile, []byte(spec), 0o644)

	s := gomcp.New("test", "1.0")
	err := ImportOpenAPI(s, specFile, OpenAPIOptions{}) // no ServerURL, should use spec
	if err != nil {
		t.Fatal(err)
	}

	c := mcptest.NewClient(t, s)
	tools := c.ListTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
}

func TestImportOpenAPI_JSONFormat(t *testing.T) {
	spec := `{"openapi":"3.0.0","info":{"title":"Test","version":"1.0"},"paths":{"/ping":{"get":{"operationId":"ping","summary":"Ping","parameters":[]}}}}`

	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.json")
	os.WriteFile(specFile, []byte(spec), 0o644)

	s := gomcp.New("test", "1.0")
	err := ImportOpenAPI(s, specFile, OpenAPIOptions{})
	if err != nil {
		t.Fatal(err)
	}

	c := mcptest.NewClient(t, s)
	tools := c.ListTools()
	if len(tools) != 1 || tools[0] != "ping" {
		t.Errorf("expected [ping], got %v", tools)
	}
}

func TestSchemaType_Empty(t *testing.T) {
	if schemaType("") != "string" {
		t.Error("empty type should default to string")
	}
	if schemaType("integer") != "integer" {
		t.Error("integer should stay integer")
	}
}

// --- gRPC helper function tests (no real connection needed) ---

func TestGrpcToolName_Default(t *testing.T) {
	name := grpcToolName("user.UserService", "GetUser", nil)
	expected := "user_service.get_user"
	if name != expected {
		t.Errorf("expected %s, got %s", expected, name)
	}
}

func TestGrpcToolName_Custom(t *testing.T) {
	name := grpcToolName("user.UserService", "GetUser", func(svc, method string) string {
		return svc + "/" + method
	})
	if name != "user.UserService/GetUser" {
		t.Errorf("unexpected: %s", name)
	}
}

func TestToSnakeCase(t *testing.T) {
	cases := map[string]string{
		"GetUser":       "get_user",
		"simpleTest":    "simple_test",
		"already_snake": "already_snake",
		"A":             "a",
	}
	for input, expected := range cases {
		got := toSnakeCase(input)
		if got != expected {
			t.Errorf("toSnakeCase(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestProtoKindToJSONType(t *testing.T) {
	cases := map[protoreflect.Kind]string{
		protoreflect.BoolKind:   "boolean",
		protoreflect.Int32Kind:  "integer",
		protoreflect.FloatKind:  "number",
		protoreflect.StringKind: "string",
		protoreflect.BytesKind:  "string",
		protoreflect.EnumKind:   "string",
	}
	for kind, expected := range cases {
		got := protoKindToJSONType(kind)
		if got != expected {
			t.Errorf("protoKindToJSONType(%v) = %q, want %q", kind, got, expected)
		}
	}
}
