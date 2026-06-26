import { useMemo } from 'react'
import { type Table } from '@tanstack/react-table'
import { Download } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { CopyButton } from '@/components/copy-button'
import { DataTableBulkActions } from '@/components/data-table'
import type { InviteCode } from './types'

type InviteCodeBulkActionsProps = {
  table: Table<InviteCode>
}

export function InviteCodeBulkActions({ table }: InviteCodeBulkActionsProps) {
  const { t } = useTranslation()
  const selectedRows = table.getFilteredSelectedRowModel().rows

  const codesToCopy = useMemo(
    () => selectedRows.map((row) => row.original.code).join('\n'),
    [selectedRows]
  )

  const handleExport = () => {
    const codes = selectedRows.map((row) => {
      const c = row.original
      return [
        c.code,
        c.is_used ? 'Used' : 'Available',
        c.used_by || '',
        c.created_time ? new Date(c.created_time * 1000).toISOString() : '',
      ].join(',')
    })
    const csv = ['Code,Status,Used By,Created Time', ...codes].join('\n')
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `invite-codes-${new Date().toISOString().slice(0, 10)}.csv`
    a.click()
    URL.revokeObjectURL(url)
    toast.success(
      t('Exported {{count}} invite codes', { count: selectedRows.length })
    )
  }

  return (
    <DataTableBulkActions table={table} entityName={t('invite code')}>
      <CopyButton
        value={codesToCopy}
        variant='outline'
        size='icon'
        className='size-8'
        tooltip={t('Copy selected codes')}
        successTooltip={t('Codes copied!')}
        aria-label={t('Copy selected codes')}
      />

      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='outline'
              size='icon'
              onClick={handleExport}
              className='size-8'
              aria-label={t('Export selected codes as CSV')}
            />
          }
        >
          <Download />
        </TooltipTrigger>
        <TooltipContent>
          <p>{t('Export selected codes as CSV')}</p>
        </TooltipContent>
      </Tooltip>
    </DataTableBulkActions>
  )
}
