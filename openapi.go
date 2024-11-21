package twopdocs

import (
	"fmt"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"google.golang.org/protobuf/compiler/protogen"
)

func NavigaSaasServer(application string, infomaker bool) *openapi3.Server {
	server := &openapi3.Server{
		URL: fmt.Sprintf(
			"https://%s-{region}.saas-{env}.navigacloud.com",
			application,
		),
		Variables: map[string]*openapi3.ServerVariable{
			"region": {
				Default:     "eu-west-1",
				Description: "the API region",
			},
			"env": {
				Default: "stage",
				Enum:    []string{"stage", "prod", "dev"},
			},
		},
	}

	if infomaker {
		server.URL = fmt.Sprintf(
			"https://%s-{region}.saas-{env}.{domain}",
			application,
		)

		server.Variables["domain"] = &openapi3.ServerVariable{
			Default:     "navigacloud.com",
			Enum:        []string{"navigacloud.com", "infomaker.io"},
			Description: "the regional top domain",
		}
	}

	return server
}

func ToOpenAPI(
	application string, version string, d Doc,
	_ *protogen.Plugin, schemaGen *SchemaGenerator,
) (openapi3.T, error) {
	var doc openapi3.T

	doc.OpenAPI = "3.0.0"
	doc.Info = &openapi3.Info{
		Title:   application + " API",
		Version: version,
	}

	doc.Components = &openapi3.Components{}
	doc.Components.Schemas = make(openapi3.Schemas)
	doc.Components.SecuritySchemes = make(openapi3.SecuritySchemes)

	bearer := openapi3.NewJWTSecurityScheme()

	doc.Components.SecuritySchemes["bearer"] = &openapi3.SecuritySchemeRef{
		Value: bearer,
	}

	requireBearer := openapi3.NewSecurityRequirement()
	requireBearer.Authenticate("bearer")

	for _, file := range d.Files {
		for _, service := range file.Services {

			doc.Tags = append(doc.Tags, &openapi3.Tag{
				Name:        service.Name,
				Description: service.Description,
			})

			for _, method := range service.Methods {
				path, op, err := createOperation(&doc, schemaGen, service, method)
				if err != nil {
					return doc, fmt.Errorf(
						"failed to create the operation for %q: %w",
						method.Name, err)
				}

				op.Tags = append(op.Tags, service.Name)

				op.Security = openapi3.NewSecurityRequirements().With(requireBearer)

				doc.AddOperation(path, http.MethodPost, op)
			}
		}

		for _, message := range file.Messages {
			_, custom := schemaGen.CustomFields[message.FullName]
			if custom {
				continue
			}

			schema, err := schemaGen.MessageSchema(message.FullName)
			if err != nil {
				return doc, fmt.Errorf(
					"failed to generate schema for %s: %w",
					message.FullName, err)
			}

			doc.Components.Schemas[schemaID(message.FullName)] = schema
		}

		for _, enum := range file.Enums {
			schema, err := schemaGen.EnumSchema(enum)
			if err != nil {
				return doc, fmt.Errorf(
					"failed to generate schema for %s: %w",
					enum.FullName, err)
			}

			doc.Components.Schemas[schemaID(enum.FullName)] = schema
		}
	}

	return doc, nil
}

func schemaID(typeName string) string {
	return typeName
}

func schemaRef(typeName string) *openapi3.SchemaRef {
	return openapi3.NewSchemaRef(
		"#/components/schemas/"+typeName, nil,
	)
}

func createOperation(
	_ *openapi3.T, _ *SchemaGenerator, service Service, method Method,
) (string, *openapi3.Operation, error) {
	op := openapi3.NewOperation()

	op.Summary = method.Name
	op.Description = method.Description

	methodPath := fmt.Sprintf("/twirp/%s/%s",
		service.FullName, method.Name)

	response := openapi3.NewResponse()
	response.Description = strPtr("Method response")

	request := openapi3.NewRequestBody()

	op.RequestBody = &openapi3.RequestBodyRef{
		Value: request.WithJSONSchemaRef(schemaRef(method.RequestType)),
	}
	op.AddResponse(http.StatusOK, response.WithJSONSchemaRef(
		schemaRef(method.ResponseType)))

	return methodPath, op, nil
}

func strPtr(str string) *string {
	return &str
}

type CustomFieldFunc func(f MessageField) (*openapi3.SchemaRef, error)

type SchemaGenerator struct {
	doc          Doc
	CustomFields map[string]CustomFieldFunc
}

func NewSchemaGenerator(doc Doc) *SchemaGenerator {
	return &SchemaGenerator{
		doc: doc,
		CustomFields: map[string]CustomFieldFunc{
			"google.protobuf.Timestamp": GoogleTimestamp,
		},
	}
}

func GoogleTimestamp(f MessageField) (*openapi3.SchemaRef, error) {
	schema := openapi3.NewSchema()
	schema.Type = &openapi3.Types{openapi3.TypeString}
	schema.Format = "date-time"
	schema.Description = f.Description

	return schema.NewRef(), nil
}

func (sg *SchemaGenerator) EnumSchema(e Enum) (*openapi3.SchemaRef, error) {
	schema := openapi3.NewSchema()

	schema.Description = e.Description
	schema.Type = &openapi3.Types{openapi3.TypeString}

	for _, v := range e.Values {
		schema.Enum = append(schema.Enum, v.Name)
	}

	return schema.NewRef(), nil
}

func (sg *SchemaGenerator) MessageSchema(typeName string) (*openapi3.SchemaRef, error) {
	schema := openapi3.NewSchema()
	schema.Type = &openapi3.Types{openapi3.TypeString}
	schema.Properties = make(openapi3.Schemas)

	msg := findMessage(sg.doc.Files, typeName)
	if msg.Name == "" {
		return nil, fmt.Errorf("unknown message type %q", typeName)
	}

	schema.Description = msg.Description

	for _, f := range msg.Fields {
		fs, err := sg.fieldSchema(f)
		if err != nil {
			return nil, fmt.Errorf("failed to generate %s (%s) schema", f.Name, f.FullType)
		}

		if f.IsRequired {
			schema.Required = append(schema.Required, f.Name)
		}

		schema.Properties[f.Name] = fs
	}

	return schema.NewRef(), nil
}

var scalarTypeMap = map[string]string{
	"int32": "integer", "int64": "integer",
	"uint32": "integer", "uint64": "integer",
	"sint32": "integer", "sint64": "integer",
	"fixed32": "integer", "fixed64": "integer",
	"sfixed32": "integer", "sfixed64": "integer",
	"string": "string",
	"bytes":  "string",
	"double": "number", "float": "number",
	"bool": "boolean",
}

func (sg *SchemaGenerator) fieldSchema(f MessageField) (*openapi3.SchemaRef, error) {
	custom, ok := sg.CustomFields[f.FullType]
	if ok {
		return custom(f)
	}

	if f.IsRepeated {
		schema := openapi3.NewSchema()
		schema.Description = f.Description
		schema.Type = &openapi3.Types{openapi3.TypeArray}

		itemField := f
		itemField.Description = ""
		itemField.IsRepeated = false

		itemSchema, err := sg.fieldSchema(itemField)
		if err != nil {
			return nil, fmt.Errorf("failed to generate array item schema: %w", err)
		}

		schema.Items = itemSchema

		return schema.NewRef(), nil
	}

	if f.IsMap {
		_, ok := scalarTypeMap[f.MapKey]
		if !ok {
			return nil, fmt.Errorf("map key must be a scalar, was: %q", f.MapKey)
		}

		schema := openapi3.NewSchema()
		schema.Description = f.Description

		schema.Type = &openapi3.Types{openapi3.TypeObject}

		itemField := f
		itemField.Description = ""
		itemField.IsMap = false
		itemField.FullType = f.MapValue

		itemSchema, err := sg.fieldSchema(itemField)
		if err != nil {
			return nil, fmt.Errorf("failed to generate array item schema: %w", err)
		}

		schema.AdditionalProperties = openapi3.AdditionalProperties{
			Schema: itemSchema,
		}

		return schema.NewRef(), nil
	}

	scalar, ok := scalarTypeMap[f.FullType]
	if ok {
		schema := openapi3.NewSchema()
		schema.Description = f.Description
		schema.Type = &openapi3.Types{scalar}

		return schema.NewRef(), nil
	}

	_, isEnum := findEnum(sg.doc.Files, f.FullType)
	if isEnum {
		return schemaRef(f.FullType), nil
	}

	msg := findMessage(sg.doc.Files, f.FullType)
	if msg.Name != "" {
		return schemaRef(f.FullType), nil
	}

	return nil, fmt.Errorf("unhandled field type %s", f.FullType)
}

func findMessage(files []File, name string) Message {
	for _, f := range files {
		for _, m := range f.Messages {
			if m.FullName == name {
				return m
			}
		}
	}

	return Message{}
}

func findEnum(files []File, name string) (Enum, bool) {
	for _, f := range files {
		for _, e := range f.Enums {
			if e.FullName == name {
				return e, true
			}
		}
	}

	return Enum{}, false
}
