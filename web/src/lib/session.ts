const sessionTokenKey = "gitsss-api-token"
const legacySessionTokenKey = "gitsss.api-token"
const selectedRepositoryKey = "gitsss-repository-id"

export function readSessionToken() {
  const token =
    window.sessionStorage.getItem(sessionTokenKey) ??
    window.sessionStorage.getItem(legacySessionTokenKey) ??
    ""
  if (token) writeSessionToken(token)
  return token
}

export function writeSessionToken(token: string) {
  const normalized = token.trim()
  if (normalized) {
    window.sessionStorage.setItem(sessionTokenKey, normalized)
  } else {
    window.sessionStorage.removeItem(sessionTokenKey)
  }
  window.sessionStorage.removeItem(legacySessionTokenKey)
}

export function readSelectedRepository() {
  return window.sessionStorage.getItem(selectedRepositoryKey) ?? ""
}

export function writeSelectedRepository(repositoryID: string) {
  if (repositoryID) {
    window.sessionStorage.setItem(selectedRepositoryKey, repositoryID)
  } else {
    window.sessionStorage.removeItem(selectedRepositoryKey)
  }
}
