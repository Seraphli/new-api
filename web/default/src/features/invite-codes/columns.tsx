import { type ColumnDef } from '@tanstack/react-table'
import { Copy, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { TableId } from '@/components/table-id'
import type { InviteCode } from './types'

export function useInviteCodesColumns(
  onDelete: (id: number) => void
): ColumnDef<InviteCode>[] {
  const { t } = useTranslation()

  const handleCopy = async (code: string) => {
    try {
      await navigator.clipboard.writeText(code)
      toast.success(t('Copied: {{code}}', { code }))
    } catch {
      toast.error(t('Copy failed'))
    }
  }

  return [
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected()}
          indeterminate={table.getIsSomePageRowsSelected()}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('Select all')}
          className='translate-y-[2px]'
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label={t('Select row')}
          className='translate-y-[2px]'
        />
      ),
      enableSorting: false,
      enableHiding: false,
      size: 40,
    },
    {
      accessorKey: 'id',
      header: t('ID'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <TableId value={row.getValue('id') as number} className='w-[60px]' />
      ),
      size: 80,
    },
    {
      accessorKey: 'code',
      header: t('Invite Code'),
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <span className='font-mono font-semibold'>
          {row.getValue('code') as string}
        </span>
      ),
      size: 200,
    },
    {
      accessorKey: 'is_used',
      header: t('Status'),
      meta: { mobileBadge: true },
      cell: ({ row }) =>
        row.getValue('is_used') ? (
          <Badge variant='destructive'>{t('Used')}</Badge>
        ) : (
          <Badge variant='secondary'>{t('Available')}</Badge>
        ),
      size: 100,
    },
    {
      accessorKey: 'created_time',
      header: t('Created'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <div className='min-w-[160px] font-mono text-sm'>
          {formatTimestampToDate(row.getValue('created_time'))}
        </div>
      ),
      size: 180,
    },
    {
      accessorKey: 'used_by',
      header: t('Used By'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (row.getValue('used_by') as string) || '-',
      size: 120,
    },
    {
      id: 'actions',
      header: () => t('Actions'),
      cell: ({ row }) => (
        <div className='flex items-center gap-1'>
          <Button
            variant='ghost'
            size='icon'
            className='size-8'
            onClick={() => handleCopy(row.original.code)}
          >
            <Copy className='h-4 w-4' />
          </Button>
          <Button
            variant='ghost'
            size='icon'
            className='size-8'
            onClick={() => onDelete(row.original.id)}
          >
            <Trash2 className='h-4 w-4' />
          </Button>
        </div>
      ),
      meta: { pinned: 'right' as const },
    },
  ]
}
