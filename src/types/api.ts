/**
 * Contrat HTTP de l'API.
 *
 * Ce fichier est destine a etre REGENERE, pas edite : quand
 * plugiit-api-go exposera sa spec OpenAPI, lancer
 *
 *   npm run api:types
 *
 * qui ecrase ce fichier depuis /openapi.json. En attendant, les quelques
 * endpoints existants sont decrits a la main dans la forme produite par
 * openapi-typescript, pour que le client soit typé des maintenant.
 */

export interface ApiError {
  code: string
  message: string
  details: Record<string, unknown>
}

export interface User {
  id: string
  email: string
  firstname: string
  lastname: string
  role: 'admin' | 'team' | 'client'
  avatar_url: string | null
}

/** Enveloppe commune a toutes les listes : la pagination est cote serveur. */
export interface Page<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface Project {
  id: string
  name: string
  status: string
  client_name: string | null
  completion: number
  updated_at: string
}

export interface paths {
  '/api/v1/auth/login': {
    post: {
      requestBody: { content: { 'application/json': { email: string; password: string } } }
      responses: {
        200: { content: { 'application/json': User } }
        401: { content: { 'application/json': ApiError } }
      }
    }
  }
  '/api/v1/auth/logout': {
    post: { responses: { 204: { content: never } } }
  }
  '/api/v1/auth/me': {
    get: {
      responses: {
        200: { content: { 'application/json': User } }
        401: { content: { 'application/json': ApiError } }
      }
    }
  }
  '/api/v1/admin/dashboard': {
    get: {
      responses: {
        200: {
          content: {
            'application/json': {
              active_projects: number
              tasks_in_progress: number
              overdue_tasks: number
              open_tickets: number
            }
          }
        }
      }
    }
  }
  '/api/v1/admin/projects': {
    get: {
      parameters: {
        query?: { page?: number; page_size?: number; search?: string; status?: string }
      }
      responses: {
        200: { content: { 'application/json': Page<Project> } }
      }
    }
  }
}
