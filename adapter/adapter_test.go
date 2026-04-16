package adapter

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/istarshine/gomcp"
	"github.com/istarshine/gomcp/mcptest"
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
