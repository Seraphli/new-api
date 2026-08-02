import { useCallback, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Loader2, Plus, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { DataTablePage, useDataTable } from '@/components/data-table'
import {
  getInviteCodes,
  generateInviteCodes,
  deleteInviteCode,
} from './api'
import { useInviteCodesColumns } from './columns'
import { InviteCodeBulkActions } from './bulk-actions'
import type { InviteCode } from './types'

export function InviteCodes() {
  const { t } = useTranslation()
  const [generateCount, setGenerateCount] = useState(5)
  const [isGenerating, setIsGenerating] = useState(false)
  const [refreshTrigger, setRefreshTrigger] = useState(0)
  const [pagination, setPagination] = useState({ pageIndex: 0, pageSize: 20 })

  const triggerRefresh = useCallback(() => {
    setRefreshTrigger((prev) => prev + 1)
  }, [])

  const handleDelete = useCallback(
    async (id: number) => {
      try {
        const res = await deleteInviteCode(id)
        if (!res.success) {
          toast.error(res.message || t('Failed to delete'))
          return
        }
        toast.success(t('Deleted'))
        triggerRefresh()
      } catch {
        toast.error(t('Failed to delete'))
      }
    },
    [t, triggerRefresh]
  )

  const columns = useInviteCodesColumns(handleDelete)

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'invite-codes',
      pagination.pageIndex + 1,
      pagination.pageSize,
      refreshTrigger,
    ],
    queryFn: async () => {
      const result = await getInviteCodes({
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      })
      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const { table } = useDataTable<InviteCode>({
    data: data?.items || [],
    columns,
    enableRowSelection: true,
    pagination,
    onPaginationChange: setPagination,
    manualPagination: true,
    totalCount: data?.total || 0,
  })

  const handleGenerate = async () => {
    setIsGenerating(true)
    try {
      const count = Math.max(1, Math.min(100, Number(generateCount) || 1))
      const res = await generateInviteCodes(count)
      if (!res.success) {
        toast.error(res.message || t('Failed to generate invite codes'))
        return
      }
      toast.success(
        t('Generated {{count}} invite codes', {
          count: res.data?.length || 0,
        })
      )
      triggerRefresh()
    } catch {
      toast.error(t('Failed to generate invite codes'))
    } finally {
      setIsGenerating(false)
    }
  }

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t('Invite Codes')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <div className='flex items-center gap-2'>
          <Input
            type='number'
            min={1}
            max={100}
            value={generateCount}
            onChange={(e) => setGenerateCount(Number(e.target.value))}
            className='w-20'
          />
          <Button onClick={handleGenerate} disabled={isGenerating}>
            {isGenerating ? (
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            ) : (
              <Plus className='mr-2 h-4 w-4' />
            )}
            {t('Generate')}
          </Button>
          <Button
            variant='outline'
            onClick={triggerRefresh}
            disabled={isFetching}
          >
            <RefreshCw className='mr-2 h-4 w-4' />
            {t('Refresh')}
          </Button>
        </div>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <DataTablePage
          table={table}
          columns={columns}
          isLoading={isLoading}
          isFetching={isFetching}
          emptyTitle={t('No invite codes')}
          emptyDescription={t(
            'No invite codes available. Generate your first invite codes to get started.'
          )}
          skeletonKeyPrefix='invite-codes-skeleton'
          applyHeaderSize
          bulkActions={<InviteCodeBulkActions table={table} />}
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
