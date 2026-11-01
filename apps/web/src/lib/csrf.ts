let token = ''

export function setCSRFToken(value: string) {
  token = value
}

export function csrfToken() {
  return token
}

export function csrfHeaders(): Record<string, string> {
  return { 'X-CSRF-Token': token }
}
