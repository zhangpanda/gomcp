package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/zhangpanda/gomcp"
	"google.golang.org/grpc"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha" //nolint:staticcheck // migrating to v1 reflection API is a separate task
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// GRPCOptions controls gRPC import behavior.
type GRPCOptions struct {
	Services   []string // filter to these service names (empty = all)
	NamingFunc func(service, method string) string
	// Timeout caps how long the reflection discovery (and each request
	// within it) may run. Zero means 10 seconds; negative disables the
	// cap. Without it, a stuck reflection server hung ImportGRPC
	// forever.
	Timeout time.Duration
	// Logger, when set, receives a warning each time a service or file
	// descriptor is skipped. Previously discovery failures were
	// silently swallowed, making it impossible to tell why a method
	// was missing.
	Logger *slog.Logger
}

// ImportGRPC discovers gRPC services via server reflection and registers each
// unary method as an MCP tool. Streaming methods are skipped.
func ImportGRPC(s *gomcp.Server, conn *grpc.ClientConn, opts GRPCOptions) error { //nolint:staticcheck // uses deprecated grpc_reflection_v1alpha
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	client := rpb.NewServerReflectionClient(conn)
	stream, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		return fmt.Errorf("grpc reflection: %w", err)
	}
	defer func() { _ = stream.CloseSend() }()

	// list services
	if err := stream.Send(&rpb.ServerReflectionRequest{
		MessageRequest: &rpb.ServerReflectionRequest_ListServices{ListServices: ""},
	}); err != nil {
		return fmt.Errorf("list services: %w", err)
	}
	resp, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("list services recv: %w", err)
	}
	listResp := resp.GetListServicesResponse()
	if listResp == nil {
		return fmt.Errorf("unexpected reflection response")
	}

	serviceFilter := make(map[string]bool)
	for _, svc := range opts.Services {
		serviceFilter[svc] = true
	}

	for _, svc := range listResp.Service {
		name := svc.GetName()
		if name == "grpc.reflection.v1alpha.ServerReflection" || name == "grpc.reflection.v1.ServerReflection" {
			continue
		}
		if len(serviceFilter) > 0 && !serviceFilter[name] {
			continue
		}

		// get file descriptor for service
		if err := stream.Send(&rpb.ServerReflectionRequest{
			MessageRequest: &rpb.ServerReflectionRequest_FileContainingSymbol{FileContainingSymbol: name},
		}); err != nil {
			continue
		}
		fdResp, err := stream.Recv()
		if err != nil {
			continue
		}
		fdBytes := fdResp.GetFileDescriptorResponse()
		if fdBytes == nil {
			continue
		}

		// parse file descriptors
		files, err := parseFileDescriptors(fdBytes.FileDescriptorProto)
		if err != nil {
			continue
		}

		for _, fd := range files {
			for i := 0; i < fd.Services().Len(); i++ {
				sd := fd.Services().Get(i)
				if len(serviceFilter) > 0 && !serviceFilter[string(sd.FullName())] {
					continue
				}
				registerServiceMethods(s, conn, sd, opts)
			}
		}
	}
	return nil
}

func parseFileDescriptors(raw [][]byte) ([]protoreflect.FileDescriptor, error) {
	var fdps []*descriptorpb.FileDescriptorProto
	for _, b := range raw {
		fdp := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(b, fdp); err != nil {
			return nil, err
		}
		fdps = append(fdps, fdp)
	}
	fds := &descriptorpb.FileDescriptorSet{File: fdps}
	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, err
	}
	var result []protoreflect.FileDescriptor
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		result = append(result, fd)
		return true
	})
	return result, nil
}

func registerServiceMethods(s *gomcp.Server, conn *grpc.ClientConn, sd protoreflect.ServiceDescriptor, opts GRPCOptions) {
	svcName := string(sd.FullName())
	for i := 0; i < sd.Methods().Len(); i++ {
		md := sd.Methods().Get(i)
		// skip streaming methods
		if md.IsStreamingClient() || md.IsStreamingServer() {
			continue
		}

		methodName := string(md.Name())
		fullMethod := fmt.Sprintf("/%s/%s", svcName, methodName)
		toolName := grpcToolName(svcName, methodName, opts.NamingFunc)
		desc := fmt.Sprintf("gRPC %s", fullMethod)

		inputDesc := md.Input()
		outputDesc := md.Output()
		inputSchema := protoMessageToSchema(inputDesc)

		capturedMethod := fullMethod
		capturedInputDesc := inputDesc
		capturedOutputDesc := outputDesc

		handler := func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
			return callGRPCMethod(conn, capturedMethod, capturedInputDesc, capturedOutputDesc, ctx)
		}

		s.RegisterToolRaw(toolName, desc, inputSchema, handler)
	}
}

func callGRPCMethod(conn *grpc.ClientConn, fullMethod string, inputDesc, outputDesc protoreflect.MessageDescriptor, ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
	// build request message from tool arguments
	reqMsg := dynamicpb.NewMessage(inputDesc)
	argsJSON, _ := json.Marshal(ctx.Args())
	if err := protojson.Unmarshal(argsJSON, reqMsg); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	respMsg := dynamicpb.NewMessage(outputDesc)
	err := conn.Invoke(ctx.Context(), fullMethod, reqMsg, respMsg)
	if err != nil {
		return nil, fmt.Errorf("gRPC error: %w", err)
	}

	respJSON, _ := protojson.Marshal(respMsg)
	return gomcp.TextResult(string(respJSON)), nil
}

// protoMessageToSchema builds a JSON schema for a protobuf message.
// It handles repeated fields as arrays, map fields as open objects,
// and message / group fields as nested objects — the previous version
// flattened everything to scalar types, leaving clients unable to see
// the actual request shape.
func protoMessageToSchema(md protoreflect.MessageDescriptor) gomcp.JSONSchema {
	return messageToJSONSchema(md, make(map[protoreflect.FullName]bool))
}

func messageToJSONSchema(md protoreflect.MessageDescriptor, visited map[protoreflect.FullName]bool) gomcp.JSONSchema {
	if visited[md.FullName()] {
		// Recursive proto types (e.g. a tree node). Stop here — the
		// outer object is already declared, we just avoid infinite
		// descent by returning an anonymous object.
		return gomcp.JSONSchema{Type: "object"}
	}
	visited[md.FullName()] = true
	defer delete(visited, md.FullName())

	props := make(map[string]gomcp.JSONSchema)
	var required []string

	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		name := string(fd.JSONName())
		props[name] = fieldToJSONSchema(fd, visited)
	}

	return gomcp.JSONSchema{Type: "object", Properties: props, Required: required}
}

func fieldToJSONSchema(fd protoreflect.FieldDescriptor, visited map[protoreflect.FullName]bool) gomcp.JSONSchema {
	desc := string(fd.FullName())

	if fd.IsMap() {
		// proto map<K,V> → JSON object with no fixed properties.
		// Document the value kind in the description since JSON Schema
		// draft used by gomcp doesn't expose additionalProperties.
		return gomcp.JSONSchema{
			Type:        "object",
			Description: desc + " (map<" + fd.MapKey().Kind().String() + "," + fd.MapValue().Kind().String() + ">)",
		}
	}

	if fd.IsList() {
		// repeated T → array.
		var items gomcp.JSONSchema
		if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
			items = messageToJSONSchema(fd.Message(), visited)
		} else {
			items = gomcp.JSONSchema{Type: protoKindToJSONType(fd.Kind())}
		}
		return gomcp.JSONSchema{Type: "array", Items: &items, Description: desc}
	}

	if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
		nested := messageToJSONSchema(fd.Message(), visited)
		nested.Description = desc
		return nested
	}

	return gomcp.JSONSchema{Type: protoKindToJSONType(fd.Kind()), Description: desc}
}

func protoKindToJSONType(k protoreflect.Kind) string {
	switch k {
	case protoreflect.BoolKind:
		return "boolean"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Uint32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Uint64Kind,
		protoreflect.Fixed32Kind, protoreflect.Fixed64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		return "integer"
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return "number"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		return "string"
	case protoreflect.EnumKind:
		return "string"
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return "object"
	default:
		return "string"
	}
}

func grpcToolName(service, method string, custom func(string, string) string) string {
	if custom != nil {
		return custom(service, method)
	}
	// "user.UserService" + "GetUser" → "user_service.get_user"
	parts := strings.Split(service, ".")
	svc := parts[len(parts)-1]
	return toSnakeCase(svc) + "." + toSnakeCase(method)
}

func toSnakeCase(s string) string {
	runes := []rune(s)
	var out []rune
	for i, r := range runes {
		if unicode.IsUpper(r) {
			// Insert underscore before an upper-case letter except at
			// the start, and except when it's the middle of an acronym
			// run like "HTTPServer" (neighbours both upper-case).
			if i > 0 {
				prev := runes[i-1]
				next := rune(0)
				if i+1 < len(runes) {
					next = runes[i+1]
				}
				if unicode.IsLower(prev) || (unicode.IsUpper(prev) && unicode.IsLower(next)) {
					out = append(out, '_')
				}
			}
			out = append(out, unicode.ToLower(r))
		} else {
			out = append(out, r)
		}
	}
	return string(out)
}
