export interface InviteCode {
  id: number
  code: string
  created_time: number
  used_time: number
  used_user_id: number
  used_by: string
  is_used: boolean
}

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface GetInviteCodesParams {
  p?: number
  page_size?: number
}

export interface GetInviteCodesResponse {
  success: boolean
  message?: string
  data?: {
    items: InviteCode[]
    total: number
    page: number
    page_size: number
  }
}
