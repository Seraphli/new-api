import { api } from '@/lib/api'
import type {
  ApiResponse,
  GetInviteCodesParams,
  GetInviteCodesResponse,
} from './types'

export async function getInviteCodes(
  params: GetInviteCodesParams = {}
): Promise<GetInviteCodesResponse> {
  const { p = 1, page_size = 20 } = params
  const res = await api.get(`/api/invite-code/?p=${p}&page_size=${page_size}`)
  return res.data
}

export async function generateInviteCodes(
  count: number
): Promise<ApiResponse<string[]>> {
  const res = await api.post('/api/invite-code/generate', { count })
  return res.data
}

export async function deleteInviteCode(
  id: number
): Promise<ApiResponse> {
  const res = await api.delete(`/api/invite-code/${id}`)
  return res.data
}
