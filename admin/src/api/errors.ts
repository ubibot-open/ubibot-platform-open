// Shared helper for turning a caught error into a message the user can
// read in their chosen language. Every page should use this instead of
// reading `.message` off an ApiError directly (that's always English --
// see server/internal/api/middleware.go's adminErrCodes).
import i18n from '../i18n'
import { ApiError } from './client'

/**
 * @param e            The value caught in a try/catch around an api.* call.
 * @param fallbackText An already-translated string (typically `t('xxx.someActionFailed')`
 *                      from the caller's own namespace) to use when `e` isn't
 *                      an ApiError (e.g. a network failure).
 */
export function apiErrorMessage(e: unknown, fallbackText: string): string {
  if (e instanceof ApiError) {
    return i18n.t(`errors:${e.code || 'error'}`, { defaultValue: e.message })
  }
  return fallbackText
}
