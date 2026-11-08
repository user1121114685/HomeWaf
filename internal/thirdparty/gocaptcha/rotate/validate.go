// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

// HomeWaf adaptation of go-captcha v2.0.5: import namespace and maintained OpenType rendering.
/**
 * @Author Awen
 * @Date 2024/06/01
 * @Email wengaolng@gmail.com
 **/

package rotate

// Validate checks if the rotation angle is within the specified range
// params:
//   - angle: Current angle
//   - dAngle: Target angle
//   - padding: Angle padding
//
// return: Whether within range
func Validate(angle, dAngle, padding int) bool {
	minAngle := 360 - padding
	maxAngle := 360 + padding
	angle += dAngle

	return angle >= minAngle && angle <= maxAngle
}

// Deprecated: As of 2.1.0, it will be removed, please use [rotate.Validate]
func CheckAngle(angle, dAngle, padding int64) bool {
	minAngle := 360 - padding
	maxAngle := 360 + padding
	angle += dAngle

	return angle >= minAngle && angle <= maxAngle
}
