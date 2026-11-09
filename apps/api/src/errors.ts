export class HttpError extends Error {
  readonly statusCode: number
  readonly code: string

  constructor(statusCode: number, code: string, message: string) {
    super(message)
    this.statusCode = statusCode
    this.code = code
  }
}

export const notFound = (what: string) => new HttpError(404, 'not_found', `${what} not found`)
export const forbidden = (message = 'Not allowed') => new HttpError(403, 'forbidden', message)
export const badRequest = (message: string) => new HttpError(400, 'bad_request', message)
export const conflict = (message: string) => new HttpError(409, 'conflict', message)
