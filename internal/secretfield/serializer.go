// Package secretfield protects explicitly marked database fields. The design
// relies on an installation's private random key, not on confidential source.
package secretfield

import (
	"context"
	"database/sql/driver"
	"fmt"
	"reflect"

	"HomeWaf/wafsec"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

const Name = "homewaf_secret"

func init() { schema.RegisterSerializer(Name, Serializer{}) }

type Serializer struct{}

func scope(field *schema.Field) string {
	return field.Schema.Table + "/" + field.DBName
}

func (Serializer) Value(_ context.Context, field *schema.Field, _ reflect.Value, value interface{}) (interface{}, error) {
	var plain []byte
	switch v := value.(type) {
	case nil:
		return nil, nil
	case string:
		plain = []byte(v)
	case []byte:
		plain = v
	default:
		return nil, fmt.Errorf("unsupported secret field type: %s", field.Name)
	}
	if len(plain) == 0 {
		return "", nil
	}
	return wafsec.EncryptBound(plain, scope(field))
}

func (Serializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, value interface{}) error {
	// Clear before decoding, including on failure; a reused model must not keep
	// a previously decrypted secret when its stored replacement is invalid.
	field.ReflectValueOf(ctx, dst).Set(reflect.Zero(field.FieldType))
	var encoded string
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		encoded = v
	case []byte:
		encoded = string(v)
	default:
		return fmt.Errorf("invalid secret field type: %s", field.Name)
	}
	if encoded == "" {
		return nil
	}
	plain, err := wafsec.DecryptBound(encoded, scope(field))
	if err != nil {
		return fmt.Errorf("cannot authenticate secret field %s", field.Name)
	}
	defer clear(plain)
	switch field.FieldType.Kind() {
	case reflect.String:
		field.ReflectValueOf(ctx, dst).SetString(string(plain))
	case reflect.Slice:
		field.ReflectValueOf(ctx, dst).SetBytes(append([]byte(nil), plain...))
	default:
		return fmt.Errorf("unsupported secret field type: %s", field.Name)
	}
	return nil
}

type protectedValue struct {
	field *schema.Field
	value interface{}
}

func (v protectedValue) Value() (driver.Value, error) {
	return (Serializer{}).Value(context.Background(), v.field, reflect.Value{}, v.value)
}

func (protectedValue) String() string { return "[protected]" }

// ProtectUpdates covers GORM's map updates, which bypass field serializers.
// Struct creates/saves use the serializer directly and keep plaintext only in
// memory. Raw SQL and SkipHooks are maintenance interfaces, not secret writers.
func ProtectUpdates(tx *gorm.DB) error {
	values, ok := tx.Statement.Dest.(map[string]interface{})
	if !ok || tx.Statement.Schema == nil {
		return nil
	}
	copyValues := make(map[string]interface{}, len(values))
	for name, value := range values {
		field := tx.Statement.Schema.LookUpField(name)
		if field != nil && field.TagSettings["SERIALIZER"] == Name {
			if _, wrapped := value.(protectedValue); !wrapped {
				value = protectedValue{field: field, value: value}
			}
		}
		copyValues[name] = value
	}
	tx.Statement.Dest = copyValues
	return nil
}
