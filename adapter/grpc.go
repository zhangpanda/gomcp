package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/istarshine/gomcp"
	"google.golang.org/grpc"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
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
}

// ImportGRPC discovers gRPC services via server reflection and registers each
// unary method as an MCP tool. Streaming methods are skipped.
func ImportGRPC(s *gomcp.Server, conn *grpc.ClientConn, opts GRPCOptions) error {
	ctx := context.Background()
	client := rpb.NewServerReflectionClient(conn)
	stream, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		return fmt.Errorf("grpc reflection: %w", err)
	}
	defer stream.CloseSend()

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
		inputSchema := protoMessageToSchema(inputDesc)

		capturedMethod := fullMethod
		capturedInputDesc := inputDesc

		handler := func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
			return callGRPCMethod(conn, capturedMethod, capturedInputDesc, ctx)
		}

		s.RegisterToolRaw(toolName, desc, inputSchema, handler)
	}
}

func callGRPCMethod(conn *grpc.ClientConn, fullMethod string, inputDesc protoreflect.MessageDescriptor, ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
	// build request message from tool arguments
	reqMsg := dynamicpb.NewMessage(inputDesc)
	argsJSON, _ := json.Marshal(ctx.Args())
	if err := protojson.Unmarshal(argsJSON, reqMsg); err != nil {
		return gomcp.ErrorResult("invalid params: " + err.Error()), nil
	}

	respMsg := dynamicpb.NewMessage(inputDesc) // placeholder, will be replaced
	err := conn.Invoke(ctx.Context(), fullMethod, reqMsg, respMsg)
	if err != nil {
		return gomcp.ErrorResult("gRPC error: " + err.Error()), nil
	}

	respJSON, _ := protojson.Marshal(respMsg)
	return gomcp.TextResult(string(respJSON)), nil
}

func protoMessageToSchema(md protoreflect.MessageDescriptor) gomcp.JSONSchema {
	props := make(map[string]gomcp.JSONSchema)
	var required []string

	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		name := string(fd.JSONName())
		prop := gomcp.JSONSchema{
			Type:        protoKindToJSONType(fd.Kind()),
			Description: string(fd.FullName()),
		}
		props[name] = prop
	}

	return gomcp.JSONSchema{Type: "object", Properties: props, Required: required}
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
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, byte(c+'a'-'A'))
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}
