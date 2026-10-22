// Spectral resolves local $refs before invoking this rule.
function objectOnly(schema) {
  if (!schema || typeof schema !== 'object' || schema.nullable) return false;
  if (schema.type && schema.type !== 'object') return false;
  if (schema.type === 'object') return true;
  if (schema.oneOf || schema.anyOf) {
    const branches = schema.oneOf ?? schema.anyOf;
    if (!branches.length || !branches.every(objectOnly)) return false;
    return true;
  }
  return Array.isArray(schema.allOf) && schema.allOf.some(objectOnly);
}

export default function objectRequest(body, _options, { path }) {
  const content = body?.content ?? {};
  if (!content['application/json'] || Object.keys(content).length !== 1) {
    return [{ message: 'Owned requests must declare only application/json.', path: [...path, 'content'] }];
  }
  if (!objectOnly(content['application/json'].schema)) {
    return [{ message: 'The resolved request schema must constrain the root to a non-null JSON object.', path: [...path, 'content', 'application/json', 'schema'] }];
  }
}
