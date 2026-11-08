// Optional Kerberos negotiation is not used by HomeWaf's local asset build.
// Reject explicitly if a future tool tries to enable it; never fake authentication.
export function createNegotiateAuth() {
  throw new Error('HomeWaf builds do not support Kerberos proxy negotiation. Use an explicitly configured supported proxy or a direct connection.');
}
