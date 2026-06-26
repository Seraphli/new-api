import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Copy, Loader2, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatTimestampToDate } from '@/lib/format'
import {
  getInviteCodes,
  generateInviteCodes,
  deleteInviteCode,
} from './api'
import type { InviteCode } from './types'

export function InviteCodes() {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [data, setData] = useState<InviteCode[]>([])
  const [page, setPage] = useState(1)
  const [pageSize] = useState(20)
  const [total, setTotal] = useState(0)
  const [generateCount, setGenerateCount] = useState(5)

  const loadData = useCallback(
    async (targetPage = page) => {
      setLoading(true)
      try {
        const res = await getInviteCodes({ p: targetPage, page_size: pageSize })
        if (!res.success) {
          toast.error(res.message || t('Failed to load invite codes'))
          return
        }
        setData(res.data?.items || [])
        setTotal(res.data?.total || 0)
        setPage(targetPage)
      } catch {
        toast.error(t('Failed to load invite codes'))
      } finally {
        setLoading(false)
      }
    },
    [page, pageSize, t]
  )

  const handleGenerate = async () => {
    setLoading(true)
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
      loadData(1)
    } catch {
      toast.error(t('Failed to generate invite codes'))
    } finally {
      setLoading(false)
    }
  }

  const handleDelete = async (id: number) => {
    setLoading(true)
    try {
      const res = await deleteInviteCode(id)
      if (!res.success) {
        toast.error(res.message || t('Failed to delete'))
        return
      }
      toast.success(t('Deleted'))
      loadData(page)
    } catch {
      toast.error(t('Failed to delete'))
    } finally {
      setLoading(false)
    }
  }

  const handleCopy = async (code: string) => {
    try {
      await navigator.clipboard.writeText(code)
      toast.success(t('Copied: {{code}}', { code }))
    } catch {
      toast.error(t('Copy failed'))
    }
  }

  useEffect(() => {
    loadData(1)
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const totalPages = Math.ceil(total / pageSize)

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
          <Button onClick={handleGenerate} disabled={loading}>
            {loading ? (
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            ) : (
              <Plus className='mr-2 h-4 w-4' />
            )}
            {t('Generate')}
          </Button>
          <Button
            variant='outline'
            onClick={() => loadData(page)}
            disabled={loading}
          >
            <RefreshCw className='mr-2 h-4 w-4' />
            {t('Refresh')}
          </Button>
        </div>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='rounded-md border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className='w-[80px]'>{t('ID')}</TableHead>
                <TableHead>{t('Invite Code')}</TableHead>
                <TableHead className='w-[100px]'>{t('Status')}</TableHead>
                <TableHead>{t('Created')}</TableHead>
                <TableHead>{t('Used By')}</TableHead>
                <TableHead className='w-[100px]'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className='text-center py-8'>
                    {loading ? t('Loading...') : t('No invite codes')}
                  </TableCell>
                </TableRow>
              ) : (
                data.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className='font-mono text-sm'>
                      {item.id}
                    </TableCell>
                    <TableCell className='font-mono font-semibold'>
                      {item.code}
                    </TableCell>
                    <TableCell>
                      {item.is_used ? (
                        <Badge variant='destructive'>{t('Used')}</Badge>
                      ) : (
                        <Badge variant='secondary'>{t('Available')}</Badge>
                      )}
                    </TableCell>
                    <TableCell className='font-mono text-sm'>
                      {item.created_time
                        ? formatTimestampToDate(item.created_time)
                        : '-'}
                    </TableCell>
                    <TableCell>{item.used_by || '-'}</TableCell>
                    <TableCell>
                      <div className='flex items-center gap-1'>
                        <Button
                          variant='ghost'
                          size='icon'
                          onClick={() => handleCopy(item.code)}
                        >
                          <Copy className='h-4 w-4' />
                        </Button>
                        <Button
                          variant='ghost'
                          size='icon'
                          onClick={() => handleDelete(item.id)}
                        >
                          <Trash2 className='h-4 w-4' />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
        {totalPages > 1 && (
          <div className='flex items-center justify-between py-4'>
            <span className='text-muted-foreground text-sm'>
              {t('Total: {{total}}', { total })}
            </span>
            <div className='flex gap-2'>
              <Button
                variant='outline'
                size='sm'
                disabled={page <= 1}
                onClick={() => loadData(page - 1)}
              >
                {t('Previous')}
              </Button>
              <Button
                variant='outline'
                size='sm'
                disabled={page >= totalPages}
                onClick={() => loadData(page + 1)}
              >
                {t('Next')}
              </Button>
            </div>
          </div>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
