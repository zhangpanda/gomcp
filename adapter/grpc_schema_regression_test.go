package adapter

import (
	"testing"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// BUG G1: protoMessageToSchema used to treat every field as a scalar
// with type = protoKindToJSONType(fd.Kind()). It ignored
// fd.IsList() (repeated fields), fd.IsMap() (map<k,v>), and the
// recursive shape of message-typed fields. Clients that relied on a
// well-formed JSON schema saw stringified or missing structure for
// anything beyond primitives.
func TestRegression_GRPCSchemaListMapNested(t *testing.T) {
	// Build a FileDescriptor by hand:
	//
	//   message Inner { string label = 1; }
	//   message Req {
	//     repeated string tags = 1;              // array of scalars
	//     map<string, int32> counters = 2;       // map<str,int32>
	//     Inner details = 3;                     // nested message
	//     repeated Inner batches = 4;            // array of messages
	//   }
	inner := &descriptorpb.DescriptorProto{
		Name: strPtr("Inner"),
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:     strPtr("label"),
				Number:   int32Ptr(1),
				Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
				Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
				JsonName: strPtr("label"),
			},
		},
	}
	// Map entries are themselves synthetic messages with map_entry=true.
	countersEntry := &descriptorpb.DescriptorProto{
		Name: strPtr("CountersEntry"),
		Field: []*descriptorpb.FieldDescriptorProto{
			{Name: strPtr("key"), Number: int32Ptr(1), Type: typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING), Label: labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL), JsonName: strPtr("key")},
			{Name: strPtr("value"), Number: int32Ptr(2), Type: typePtr(descriptorpb.FieldDescriptorProto_TYPE_INT32), Label: labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL), JsonName: strPtr("value")},
		},
		Options: &descriptorpb.MessageOptions{MapEntry: boolPtr(true)},
	}
	req := &descriptorpb.DescriptorProto{
		Name:       strPtr("Req"),
		NestedType: []*descriptorpb.DescriptorProto{countersEntry},
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:     strPtr("tags"),
				Number:   int32Ptr(1),
				Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
				Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED),
				JsonName: strPtr("tags"),
			},
			{
				Name:     strPtr("counters"),
				Number:   int32Ptr(2),
				Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
				Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED),
				TypeName: strPtr(".pkg.Req.CountersEntry"),
				JsonName: strPtr("counters"),
			},
			{
				Name:     strPtr("details"),
				Number:   int32Ptr(3),
				Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
				Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
				TypeName: strPtr(".pkg.Inner"),
				JsonName: strPtr("details"),
			},
			{
				Name:     strPtr("batches"),
				Number:   int32Ptr(4),
				Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
				Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED),
				TypeName: strPtr(".pkg.Inner"),
				JsonName: strPtr("batches"),
			},
		},
	}
	fdp := &descriptorpb.FileDescriptorProto{
		Name:        strPtr("req.proto"),
		Syntax:      strPtr("proto3"),
		Package:     strPtr("pkg"),
		MessageType: []*descriptorpb.DescriptorProto{inner, req},
	}
	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{fdp}})
	if err != nil {
		t.Fatal(err)
	}
	fd, err := files.FindFileByPath("req.proto")
	if err != nil {
		t.Fatal(err)
	}
	var reqMD protoreflect.MessageDescriptor
	for i := 0; i < fd.Messages().Len(); i++ {
		m := fd.Messages().Get(i)
		if m.Name() == "Req" {
			reqMD = m
		}
	}
	if reqMD == nil {
		t.Fatal("Req message descriptor not found")
	}

	schema := protoMessageToSchema(reqMD)
	if schema.Type != "object" {
		t.Fatalf("root type = %q, want object", schema.Type)
	}

	// tags: repeated string -> array of string
	tags, ok := schema.Properties["tags"]
	if !ok {
		t.Fatal("missing 'tags' property")
	}
	if tags.Type != "array" {
		t.Fatalf("tags.type = %q, want array", tags.Type)
	}
	if tags.Items == nil || tags.Items.Type != "string" {
		t.Fatalf("tags.items should be {type: string}, got %+v", tags.Items)
	}

	// counters: map<string,int32> -> object with descriptive hint
	counters, ok := schema.Properties["counters"]
	if !ok {
		t.Fatal("missing 'counters' property")
	}
	if counters.Type != "object" {
		t.Fatalf("counters.type = %q, want object", counters.Type)
	}

	// details: Inner -> object with label property
	details, ok := schema.Properties["details"]
	if !ok {
		t.Fatal("missing 'details' property")
	}
	if details.Type != "object" {
		t.Fatalf("details.type = %q, want object", details.Type)
	}
	if _, ok := details.Properties["label"]; !ok {
		t.Fatalf("details should have nested 'label' property, got %+v", details.Properties)
	}

	// batches: repeated Inner -> array of object{label}
	batches, ok := schema.Properties["batches"]
	if !ok {
		t.Fatal("missing 'batches' property")
	}
	if batches.Type != "array" || batches.Items == nil || batches.Items.Type != "object" {
		t.Fatalf("batches should be array of object, got %+v", batches)
	}
	if _, ok := batches.Items.Properties["label"]; !ok {
		t.Fatalf("batches.items should have 'label', got %+v", batches.Items.Properties)
	}
}

func strPtr(s string) *string { return &s }
func int32Ptr(n int32) *int32 { return &n }
func boolPtr(b bool) *bool    { return &b }
func typePtr(t descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto_Type {
	return &t
}
func labelPtr(l descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto_Label {
	return &l
}
