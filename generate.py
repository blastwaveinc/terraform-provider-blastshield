#!/usr/bin/env python3
# Copyright 2026 BlastWave, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
OpenAPI to Terraform Provider Code Generator

Generates Go code for a Terraform provider from an OpenAPI specification.
Uses Jinja2 templates for code generation.
"""

import argparse
import json
import os
import re
from dataclasses import dataclass, field

from jinja2 import Environment, FileSystemLoader

# Configuration - skip these tags/resources
SKIP_TAGS = {"API Keys", "Audit", "APIKeys"}  # API keys don't make sense in Terraform

# Defaults
DEFAULT_SPEC = "openapi.json"
DEFAULT_OUTPUT_DIR = "internal/provider/generated"
DEFAULT_PACKAGE = "generated"
TEMPLATES_DIR = "codegen-templates"

# =============================================================================
# CUSTOMIZATIONS
# =============================================================================

# Resources that have a separate /resource/{id}/groups endpoint for group membership
RESOURCES_WITH_GROUPS = {"Node", "Endpoint"}

# Resources where POST response should be stored as base64-encoded JSON
STORE_POST_RESPONSE = {"Node"}

# Field in POST response that contains the entity ID (for GET after POST)
POST_RESPONSE_ID_FIELD = {
    "Node": "node_id",
}

# Fields that are required by the API but accept null for auto-assignment
NULLABLE_REQUIRED_FIELDS = {
    "Endpoint": ["address"],
}


@dataclass
class FieldInfo:
    name: str  # Go field name
    json_name: str  # JSON field name
    tf_name: str  # Terraform attribute name
    go_type: str  # Go type for client structs
    tf_type: str  # Terraform schema type (String, Int64, Bool, List, Map)
    tf_element_type: str  # For lists/maps, the element type
    required: bool = False
    computed: bool = False
    optional: bool = False
    sensitive: bool = False
    is_pointer: bool = False
    is_list: bool = False
    is_map: bool = False
    is_nested: bool = False
    nested_fields: list = field(default_factory=list)
    nested_ref: str = ""
    description: str = ""


@dataclass
class QueryParam:
    name: str
    tf_name: str
    go_name: str  # Go field name
    go_type: str
    tf_type: str
    is_list: bool = False
    description: str = ""


@dataclass
class ResourceInfo:
    name: str  # e.g., "Node", "Group"
    plural: str  # e.g., "Nodes", "Groups"
    tf_name: str  # e.g., "node", "group"
    tf_plural_name: str  # e.g., "nodes", "policies"
    path: str  # e.g., "/nodes/"
    id_type: str  # "string" or "int64"
    id_field: str  # "id" or "ID"
    fields: list  # List of FieldInfo
    create_fields: list  # Fields for create request
    query_params: list  # Query parameters for list endpoint
    has_groups: bool = False
    store_post_response: bool = False
    post_id_field: str = "id"


def to_go_name(name: str) -> str:
    """Convert snake_case or kebab-case to PascalCase."""
    parts = re.split(r'[_-]', name)
    result = ''.join(word.capitalize() for word in parts)
    for old, new in [("Id", "ID"), ("Idp", "IDP"), ("Dns", "DNS"),
                     ("Api", "API"), ("Ip", "IP"), ("Ha", "HA"),
                     ("Gw", "GW"), ("Fw", "FW")]:
        result = result.replace(old, new)
    return result


def to_tf_name(name: str) -> str:
    """Convert to snake_case for Terraform. Handles PascalCase, snake_case, and kebab-case."""
    # Insert underscore before uppercase letters that follow a lowercase letter or digit
    s = re.sub(r'([a-z0-9])([A-Z])', r'\1_\2', name)
    return s.lower().replace("-", "_")


def to_tf_plural_name(name: str) -> str:
    """Convert to plural snake_case for Terraform."""
    name = to_tf_name(name)
    if name.endswith("y"):
        return name[:-1] + "ies"
    return name + "s"


def parse_openapi_type(prop: dict, name: str, schemas: dict) -> FieldInfo:
    """Convert OpenAPI property to FieldInfo."""
    field_info = FieldInfo(
        name=to_go_name(name),
        json_name=name,
        tf_name=to_tf_name(name),
        go_type="string",
        tf_type="String",
        tf_element_type="",
        description=prop.get("description", ""),
    )

    prop_type = prop.get("type")
    any_of = prop.get("anyOf", [])

    if any_of:
        non_null = [t for t in any_of if t.get("type") != "null"]
        if non_null:
            prop = non_null[0]
            prop_type = prop.get("type")
            field_info.is_pointer = True

    if "$ref" in prop:
        ref_name = prop["$ref"].split("/")[-1]
        if ref_name in schemas:
            ref_schema = schemas[ref_name]
            if ref_schema.get("type") == "string" and "enum" in ref_schema:
                field_info.go_type = "*string" if field_info.is_pointer else "string"
                field_info.tf_type = "String"
                field_info.description = ref_schema.get("description", "")
                return field_info
            field_info.nested_ref = ref_name
            field_info.is_nested = True
            field_info.go_type = f"*{ref_name}" if field_info.is_pointer else ref_name
            field_info.tf_type = "Object"
            nested_props = ref_schema.get("properties", {})
            nested_required = ref_schema.get("required", [])
            for nested_name, nested_prop in nested_props.items():
                nested_field = parse_openapi_type(nested_prop, nested_name, schemas)
                nested_field.required = nested_name in nested_required
                field_info.nested_fields.append(nested_field)
        return field_info

    if prop_type == "string":
        field_info.go_type = "*string" if field_info.is_pointer else "string"
        field_info.tf_type = "String"
    elif prop_type == "integer":
        field_info.go_type = "*int64" if field_info.is_pointer else "int64"
        field_info.tf_type = "Int64"
    elif prop_type == "boolean":
        field_info.go_type = "*bool" if field_info.is_pointer else "bool"
        field_info.tf_type = "Bool"
    elif prop_type == "array":
        field_info.is_list = True
        items = prop.get("items", {})

        items_any_of = items.get("anyOf", [])
        if items_any_of:
            for item_type in items_any_of:
                if "$ref" in item_type:
                    items = item_type
                    break
                elif item_type.get("type") not in (None, "null"):
                    items = item_type

        items_type = items.get("type")
        if items_type == "string":
            field_info.go_type = "[]string"
            field_info.tf_element_type = "types.StringType"
        elif items_type == "integer":
            field_info.go_type = "[]int64"
            field_info.tf_element_type = "types.Int64Type"
        elif "$ref" in items:
            ref_name = items["$ref"].split("/")[-1]
            if ref_name in schemas:
                nested_schema = schemas[ref_name]
                if nested_schema.get("type") == "string" and "enum" in nested_schema:
                    field_info.go_type = "[]string"
                    field_info.tf_element_type = "types.StringType"
                elif nested_schema.get("type") == "integer":
                    field_info.go_type = "[]int64"
                    field_info.tf_element_type = "types.Int64Type"
                else:
                    field_info.nested_ref = ref_name
                    field_info.is_nested = True
                    field_info.go_type = f"[]{ref_name}"
                    field_info.tf_element_type = ref_name
                    nested_props = nested_schema.get("properties", {})
                    nested_required = nested_schema.get("required", [])
                    for nested_name, nested_prop in nested_props.items():
                        nested_field = parse_openapi_type(nested_prop, nested_name, schemas)
                        nested_field.required = nested_name in nested_required
                        field_info.nested_fields.append(nested_field)
        else:
            field_info.go_type = "[]interface{}"
            field_info.tf_element_type = "types.StringType"
        field_info.tf_type = "List"
    elif prop_type == "object":
        additional = prop.get("additionalProperties", {})
        if additional:
            field_info.is_map = True
            add_type = additional.get("type", "string")
            if add_type == "string":
                field_info.go_type = "map[string]string"
                field_info.tf_element_type = "types.StringType"
            else:
                field_info.go_type = "map[string]interface{}"
                field_info.tf_element_type = "types.StringType"
            field_info.tf_type = "Map"
        else:
            field_info.go_type = "map[string]interface{}"
            field_info.tf_type = "Map"
            field_info.tf_element_type = "types.StringType"

    if name in ("password", "token", "secret", "registration_token", "key", "key_hash"):
        field_info.sensitive = True

    return field_info


def parse_schema_fields(spec: dict, schema_name: str) -> list[FieldInfo]:
    """Parse an OpenAPI schema and return field info."""
    schemas = spec.get("components", {}).get("schemas", {})
    schema = schemas.get(schema_name, {})

    fields = []
    properties = schema.get("properties", {})
    required = schema.get("required", [])

    for prop_name, prop_def in properties.items():
        if prop_name in ("status", "settings"):
            continue
        field_info = parse_openapi_type(prop_def, prop_name, schemas)
        field_info.required = prop_name in required
        fields.append(field_info)

    return fields


def parse_query_params(spec: dict, path: str) -> list[QueryParam]:
    """Parse query parameters for a list endpoint."""
    params = []
    path_info = spec.get("paths", {}).get(path, {})
    method_info = path_info.get("get", {})

    for param in method_info.get("parameters", []):
        if param.get("in") != "query":
            continue

        name = param["name"]
        if "." in name:
            continue

        schema = param.get("schema", {})
        param_type = schema.get("type", "string")

        is_list = param_type == "array"
        if is_list:
            items = schema.get("items", {})
            param_type = items.get("type", "string")

        go_type = "string"
        tf_type = "String"
        if param_type == "integer":
            go_type = "int64"
            tf_type = "Int64"
        elif param_type == "boolean":
            go_type = "bool"
            tf_type = "Bool"

        params.append(QueryParam(
            name=name,
            tf_name=to_tf_name(name),
            go_name=to_go_name(name),
            go_type=go_type,
            tf_type=tf_type,
            is_list=is_list,
            description=param.get("description", ""),
        ))

    return params


def find_resources(spec: dict) -> list[ResourceInfo]:
    """Find all resources from OpenAPI paths."""
    resources = []
    paths = spec.get("paths", {})

    resource_paths = {}
    for path, methods in paths.items():
        if "{" in path:
            continue

        for method, info in methods.items():
            if method not in ("get", "post"):
                continue

            tags = info.get("tags", [])
            if not tags or tags[0] in SKIP_TAGS:
                continue

            tag = tags[0]
            if tag not in resource_paths:
                resource_paths[tag] = {"base_path": path, "methods": {}}

            if method == "post":
                request_body = info.get("requestBody", {})
                content = request_body.get("content", {}).get("application/json", {})
                schema = content.get("schema", {})
                if schema.get("type") == "array":
                    continue

            resource_paths[tag]["methods"][method] = info

    for tag, info in resource_paths.items():
        base_path = info["base_path"]
        methods = info["methods"]

        name = tag.rstrip("s")
        if name.endswith("ie"):
            name = name[:-2] + "y"
        if name == "Proxie":
            name = "Proxy"
        name = name.replace(" ", "")

        entity_schema = None
        if "get" in methods:
            response = methods["get"].get("responses", {}).get("200", {})
            content = response.get("content", {}).get("application/json", {})
            schema = content.get("schema", {})
            if schema.get("type") == "array":
                items = schema.get("items", {})
                if "$ref" in items:
                    entity_schema = items["$ref"].split("/")[-1]

        create_schema = None
        if "post" in methods:
            request_body = methods["post"].get("requestBody", {})
            content = request_body.get("content", {}).get("application/json", {})
            schema = content.get("schema", {})
            if "$ref" in schema:
                create_schema = schema["$ref"].split("/")[-1]

        if not entity_schema:
            continue

        fields = parse_schema_fields(spec, entity_schema)
        create_fields = parse_schema_fields(spec, create_schema) if create_schema else []

        id_field = next((f for f in fields if f.json_name == "id"), None)
        id_type = "int64"
        if id_field:
            if "string" in id_field.go_type:
                id_type = "string"

        query_params = parse_query_params(spec, base_path)

        create_field_names = {f.json_name for f in create_fields}
        required_create = {f.json_name for f in create_fields if f.required}
        nullable_required = set(NULLABLE_REQUIRED_FIELDS.get(name, []))

        for fld in fields:
            fld.required = False
            fld.optional = False
            fld.computed = False

            if fld.json_name == "id":
                fld.computed = True
            elif fld.json_name in nullable_required:
                fld.optional = True
                fld.computed = True
            elif fld.json_name in required_create:
                fld.required = True
            elif fld.json_name in create_field_names:
                fld.optional = True
                fld.computed = True
            else:
                fld.computed = True

        has_groups = name in RESOURCES_WITH_GROUPS
        store_post_response = name in STORE_POST_RESPONSE
        post_id_field = POST_RESPONSE_ID_FIELD.get(name, "id")

        resource = ResourceInfo(
            name=name,
            plural=tag.replace(" ", ""),
            tf_name=to_tf_name(name.replace(" ", "_")),
            tf_plural_name=to_tf_plural_name(name.replace(" ", "_")),
            path=base_path,
            id_type=id_type,
            id_field="ID" if id_type == "string" else "ID",
            fields=fields,
            create_fields=create_fields,
            query_params=query_params,
            has_groups=has_groups,
            store_post_response=store_post_response,
            post_id_field=post_id_field,
        )
        resources.append(resource)

    return resources


def main():
    parser = argparse.ArgumentParser(description="Generate Terraform provider code from OpenAPI spec")
    parser.add_argument("--spec", default=DEFAULT_SPEC, help="Path to OpenAPI spec JSON file")
    parser.add_argument("--output-dir", default=DEFAULT_OUTPUT_DIR, help="Output directory for generated Go code")
    parser.add_argument("--package", default=DEFAULT_PACKAGE, help="Go package name for generated code")
    args = parser.parse_args()

    spec_path = args.spec
    output_dir = args.output_dir
    package_name = args.package

    # Load OpenAPI spec
    with open(spec_path) as f:
        spec = json.load(f)

    api_version = spec.get("info", {}).get("version", "unknown")

    # Find resources
    resources = find_resources(spec)

    print(f"Found {len(resources)} resources (API version {api_version}, package {package_name}):")
    for r in resources:
        print(f"  - {r.name} ({r.path})")

    # Set up Jinja2 environment
    env = Environment(
        loader=FileSystemLoader(TEMPLATES_DIR),
        trim_blocks=True,
        lstrip_blocks=True,
    )

    # Create output directory
    os.makedirs(output_dir, exist_ok=True)

    # Collect nested types for types.go
    nested_types = {}
    for r in resources:
        for f in r.fields:
            if f.is_nested and f.is_list and f.nested_fields and f.nested_ref:
                if f.nested_ref not in nested_types:
                    nested_types[f.nested_ref] = f.nested_fields
        for f in r.create_fields:
            if f.is_nested and f.is_list and f.nested_fields and f.nested_ref:
                if f.nested_ref not in nested_types:
                    nested_types[f.nested_ref] = f.nested_fields

    # Check if resources have tags (for test generation)
    for r in resources:
        r.has_tags = any(f.json_name == "tags" for f in r.create_fields)

    # Generate shared files
    has_groups = any(r.has_groups for r in resources)
    generated_files = [
        ("schemas.go.j2", os.path.join(output_dir, "schemas.go"), {"resources": resources, "package_name": package_name}),
        ("types.go.j2", os.path.join(output_dir, "types.go"), {"resources": resources, "nested_types": nested_types, "package_name": package_name}),
        ("client.go.j2", os.path.join(output_dir, "client.go"), {"package_name": package_name}),
        ("helpers.go.j2", os.path.join(output_dir, "helpers.go"), {"has_groups": has_groups, "package_name": package_name}),
        ("register.go.j2", os.path.join(output_dir, "register.go"), {"resources": resources, "api_version": api_version, "package_name": package_name}),
        ("test_helpers.go.j2", os.path.join(output_dir, "test_helpers_test.go"), {"package_name": package_name}),
        ("resource_test.go.j2", os.path.join(output_dir, "resources_test.go"), {"resources": resources, "package_name": package_name}),
    ]
    for template_name, output_path, context in generated_files:
        template = env.get_template(template_name)
        content = template.render(**context)
        with open(output_path, "w") as f:
            f.write(content)
        print(f"Generated {output_path}")

    # Generate resource and data source files
    for r in resources:
        # Prepare template context
        nullable_required = set(NULLABLE_REQUIRED_FIELDS.get(r.name, []))
        required_fields = [f for f in r.create_fields if f.required and not f.is_nested and f.json_name not in nullable_required]
        optional_fields = [f for f in r.create_fields if not f.required and not f.is_nested and f.json_name not in nullable_required]
        nullable_required_fields = [f for f in r.create_fields if f.json_name in nullable_required]
        required_list_fields = [f for f in r.create_fields if f.required and f.is_list and not f.is_nested]
        nested_list_fields = [f for f in r.create_fields if f.is_nested and f.is_list and f.nested_fields]

        has_nested_list_fields = (
            any(f.is_nested and f.is_list and f.nested_fields for f in r.fields) or
            any(f.is_nested and f.is_list and f.nested_fields for f in r.create_fields)
        )

        id_value_method = "ValueString()" if r.id_type == "string" else "ValueInt64()"
        id_format = "%s" if r.id_type == "string" else "%d"
        id_set = "types.StringValue" if r.id_type == "string" else "types.Int64Value"

        # Generate resource file
        template = env.get_template("resource.go.j2")
        content = template.render(
            resource=r,
            required_fields=required_fields,
            optional_fields=optional_fields,
            nullable_required_fields=nullable_required_fields,
            required_list_fields=required_list_fields,
            nested_list_fields=nested_list_fields,
            has_nested_list_fields=has_nested_list_fields,
            id_value_method=id_value_method,
            id_format=id_format,
            id_set=id_set,
            package_name=package_name,
        )
        path = os.path.join(output_dir, f"{r.tf_name}_resource.go")
        with open(path, "w") as f:
            f.write(content)
        print(f"Generated {path}")

        # Generate data source file
        template = env.get_template("data_source.go.j2")
        content = template.render(
            resource=r,
            id_value_method=id_value_method,
            id_format=id_format,
            package_name=package_name,
        )
        path = os.path.join(output_dir, f"{r.tf_name}_data_source.go")
        with open(path, "w") as f:
            f.write(content)
        print(f"Generated {path}")

    print("\nDone! Generated files are in:", output_dir)


if __name__ == "__main__":
    main()
