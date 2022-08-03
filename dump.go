package twopdocs

import (
	"regexp"
	"strconv"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Doc struct {
	Files []File `json:"files"`
}

type File struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Services    []Service `json:"services"`
	Messages    []Message `json:"messages"`
	Enums       []Enum    `json:"enums"`
}

type Enum struct {
	Name        string      `json:"name"`
	FullName    string      `json:"fullName"`
	Description string      `json:"description"`
	Values      []EnumValue `json:"values"`
}

type EnumValue struct {
	Name        string `json:"name"`
	Number      string `json:"number"`
	Description string `json:"description"`
}

type Message struct {
	Name        string         `json:"name"`
	FullName    string         `json:"fullName"`
	Description string         `json:"description"`
	Fields      []MessageField `json:"fields"`
}

type MessageField struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	FullType    string `json:"fullType"`
	IsRepeated  bool   `json:"isRepeated,omitempty"`
	IsMap       bool   `json:"isMap"`
	IsRequired  bool   `json:"isRequired"`
	MapKey      string `json:"mapKey,omitempty"`
	MapValue    string `json:"mapValue,omitempty"`
}

type Service struct {
	Name        string   `json:"name"`
	FullName    string   `json:"fullName"`
	Description string   `json:"description"`
	Methods     []Method `json:"methods"`
}

type Method struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	RequestType  string `json:"requestType"`
	ResponseType string `json:"responseType"`
}

func StructureDump(gen *protogen.Plugin) Doc {
	var doc Doc

	for _, f := range gen.Files {
		file := File{
			Name: *f.Proto.Name,
		}

		for _, m := range f.Messages {
			file.Messages = append(file.Messages, readMessage(m))
		}

		for _, s := range f.Services {
			file.Services = append(file.Services, readService(s))
		}

		for _, e := range f.Enums {
			file.Enums = append(file.Enums, readEnum(e))
		}

		doc.Files = append(doc.Files, file)
	}

	return doc
}

func readEnum(e *protogen.Enum) Enum {
	enum := Enum{
		Name:        string(e.Desc.Name()),
		FullName:    string(e.Desc.FullName()),
		Description: whitewash(string(e.Comments.Leading)),
	}

	for _, v := range e.Values {
		value := EnumValue{
			Name:        string(v.Desc.Name()),
			Number:      strconv.Itoa(int(v.Desc.Number())),
			Description: whitewash(string(v.Comments.Leading)),
		}

		enum.Values = append(enum.Values, value)
	}

	return enum
}

func readService(s *protogen.Service) Service {
	service := Service{
		Name:        string(s.Desc.Name()),
		FullName:    string(s.Desc.FullName()),
		Description: whitewash(string(s.Comments.Leading)),
	}

	for _, m := range s.Methods {
		method := Method{
			Name:         string(m.Desc.Name()),
			Description:  whitewash(string(m.Comments.Leading)),
			RequestType:  string(m.Input.Desc.FullName()),
			ResponseType: string(m.Output.Desc.FullName()),
		}

		service.Methods = append(service.Methods, method)
	}

	return service
}

func readMessage(m *protogen.Message) Message {
	message := Message{
		Name:        string(m.Desc.Name()),
		FullName:    string(m.Desc.FullName()),
		Description: whitewash(string(m.Comments.Leading)),
	}

	for _, f := range m.Fields {
		t, ft := resolveTypes(f.Desc)

		trailing := whitewash(string(f.Comments.Trailing))
		isRequired := strings.HasPrefix(trailing, "required")
		if isRequired {
			trailing = strings.TrimPrefix(trailing, "required")
		}

		desc := whitewash(string(f.Comments.Leading) + " " + trailing)

		field := MessageField{
			Name:        string(f.Desc.Name()),
			Description: desc,
			Type:        t,
			FullType:    ft,
			IsRepeated:  f.Desc.Cardinality() == protoreflect.Repeated,
			IsMap:       f.Desc.IsMap(),
			IsRequired:  isRequired,
		}

		if field.IsMap {
			field.Type = "map"

			_, kt := resolveTypes(f.Desc.MapKey())
			_, vt := resolveTypes(f.Desc.MapValue())

			field.MapKey = kt
			field.MapValue = vt
			field.IsRepeated = false
		}

		message.Fields = append(message.Fields, field)
	}

	return message
}

var singleNewline = regexp.MustCompile(`\n[ ]+`)

func whitewash(str string) string {
	str = strings.TrimSpace(str)
	str = singleNewline.ReplaceAllLiteralString(str, " ")

	return str
}

func resolveTypes(f protoreflect.FieldDescriptor) (string, string) {
	switch f.Kind() {
	case protoreflect.MessageKind:
		msg := f.Message()
		return string(msg.Name()), string(msg.FullName())
	case protoreflect.EnumKind:
		enum := f.Enum()
		return "enum", string(enum.FullName())
	}

	return f.Kind().String(), f.Kind().String()
}
