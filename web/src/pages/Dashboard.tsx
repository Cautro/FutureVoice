import { useEffect, useMemo, useState } from 'react'
import { requestsApi } from '../api/requests'
import { Badge, Empty, ErrorBanner, Loading, MetricCard, Topbar } from '../components/UI'

const STATUS_LABELS: Record<string, string> = {
  open: 'Новые',
  in_progress: 'В работе',
  waiting_for_user: 'Ожидают ответа',
  closed: 'Завершены',
  rejected: 'Отклонены',
}

const STATUS_COLORS: Record<string, string> = {
  open: '#E6B65B',
  in_progress: '#55B9F1',
  waiting_for_user: '#A991F0',
  closed: '#3CC99C',
  rejected: '#FA7E89',
}

const TYPE_LABELS: Record<string, string> = {
  complaint: 'Жалобы',
  offer: 'Предложения',
  '3dPrintOffer': '3D-печать',
}

interface DailyPoint {
  date: string
  created: number
  resolved: number
}

interface AnalyticsData {
  from: string
  to: string
  total_requests: number
  open_requests: number
  in_progress_requests: number
  waiting_for_user_requests?: number
  closed_requests: number
  rejected_requests?: number
  average_resolution_hours?: number | null
  resolution_rate?: number
  daily?: DailyPoint[]
  by_status?: { key: string; count: number }[]
  by_type?: { key: string; count: number }[]
}

function SvgLineChart({ points }: { points: DailyPoint[] }) {
  const width = 720
  const height = 210
  const padX = 18
  const padY = 18

  const values = points.flatMap((p) => [p.created, p.resolved])
  const max = Math.max(1, ...values)

  const makeCoords = (key: 'created' | 'resolved') =>
    points.map((point, index) => ({
      x: padX + (points.length <= 1 ? 0 : (index * (width - padX * 2)) / (points.length - 1)),
      y: height - padY - (point[key] / max) * (height - padY * 2),
      value: point[key],
      date: point.date,
    }))

  const created = makeCoords('created')
  const closed = makeCoords('resolved')

  const pathFor = (coords: { x: number; y: number }[]) =>
    coords.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`).join(' ')

  return (
    <div className="chart-wrap">
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Динамика создания заявок">
        {[0, 1, 2, 3].map((line) => {
          const y = padY + (line * (height - padY * 2)) / 3
          return <line key={line} x1="0" x2={width} y1={y} y2={y} className="chart-gridline" />
        })}
        <path d={pathFor(created)} className="chart-line" />
        <path d={pathFor(closed)} className="chart-line chart-line-closed" />
        {created.map((p, i) => (
          <circle key={`c-${i}`} cx={p.x} cy={p.y} r="3" className="chart-dot" />
        ))}
        {closed.map((p, i) => (
          <circle key={`r-${i}`} cx={p.x} cy={p.y} r="3" className="chart-dot chart-dot-closed" />
        ))}
      </svg>
      <div className="chart-axis">
        <span>{points[0]?.date?.slice(5) || ''}</span>
        <span>{points[Math.floor(points.length / 2)]?.date?.slice(5) || ''}</span>
        <span>{points.at(-1)?.date?.slice(5) || ''}</span>
      </div>
      <div className="chart-legend">
        <span>
          <i className="legend-dot created-dot" /> Создано
        </span>
        <span>
          <i className="legend-dot closed-dot" /> С решением
        </span>
      </div>
    </div>
  )
}

function TypeBars({ items }: { items: { key: string; count: number }[] }) {
  const max = Math.max(1, ...items.map((i) => i.count))

  return (
    <div className="type-bars">
      {items.length === 0 && <p className="muted">За выбранный период обращений нет.</p>}
      {items.map((item) => (
        <div className="type-bar-row" key={item.key}>
          <div className="type-bar-label">
            <span>{TYPE_LABELS[item.key] || item.key}</span>
            <strong>{item.count}</strong>
          </div>
          <div className="bar-track">
            <div
              className="bar-fill"
              style={{ width: `${Math.max(2, (item.count / max) * 100)}%` }}
            />
          </div>
        </div>
      ))}
    </div>
  )
}

function StatusDonut({ data }: { data: { key: string; count: number }[] }) {
  const total = data.reduce((sum, item) => sum + item.count, 0)
  let start = 0

  const segments = data.map((item) => {
    const end = start + (total ? (item.count / total) * 100 : 0)
    const segment = `${STATUS_COLORS[item.key] || '#98A2B3'} ${start}% ${end}%`
    start = end
    return segment
  })

  return (
    <div className="donut-content">
      <div
        className="donut"
        style={{ background: total ? `conic-gradient(${segments.join(',')})` : '#20314B' }}
      >
        <div className="donut-hole">
          <strong>{total}</strong>
          <span>всего</span>
        </div>
      </div>
      <div className="donut-legend">
        {data.map((item) => (
          <div className="legend-row" key={item.key}>
            <span
              className="legend-dot"
              style={{ background: STATUS_COLORS[item.key] || '#98A2B3' }}
            />
            <span>{STATUS_LABELS[item.key] || item.key}</span>
            <strong>{item.count}</strong>
          </div>
        ))}
        {!data.length && <p className="muted">Данных пока нет.</p>}
      </div>
    </div>
  )
}

// --- PNG export (оставил как было, только чуть подчистил) ---

function roundedRect(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  width: number,
  height: number,
  radius: number,
  fill = true
) {
  ctx.beginPath()
  ctx.roundRect(x, y, width, height, radius)
  if (fill) ctx.fill()
  else ctx.stroke()
}

function drawTitle(ctx: CanvasRenderingContext2D, title: string, subtitle: string, x: number, y: number) {
  ctx.fillStyle = '#EAF2FF'
  ctx.font = 'bold 30px Arial'
  ctx.fillText(title, x, y)
  ctx.fillStyle = '#8FA2C0'
  ctx.font = '17px Arial'
  ctx.fillText(subtitle, x, y + 30)
}

function drawDistribution(
  ctx: CanvasRenderingContext2D,
  items: { key: string; count: number }[],
  labels: Record<string, string>,
  colors: Record<string, string>,
  x: number,
  y: number,
  width: number,
  rowGap = 58
) {
  const max = Math.max(1, ...items.map((i) => i.count))
  const total = items.reduce((s, i) => s + i.count, 0)

  items.forEach((item, index) => {
    const rowY = y + index * rowGap
    const count = Number(item.count || 0)

    ctx.fillStyle = '#A9B9D1'
    ctx.font = '19px Arial'
    ctx.fillText(labels[item.key] || item.key, x, rowY)

    ctx.fillStyle = '#E4EEFC'
    ctx.font = 'bold 19px Arial'
    ctx.textAlign = 'right'
    ctx.fillText(
      `${count} · ${Math.round((count / Math.max(1, total)) * 100)}%`,
      x + width,
      rowY
    )
    ctx.textAlign = 'left'

    ctx.fillStyle = '#20314B'
    roundedRect(ctx, x, rowY + 13, width, 13, 7)
    ctx.fillStyle = colors[item.key] || '#6685C8'
    roundedRect(ctx, x, rowY + 13, Math.max(4, (width * count) / max), 13, 7)
  })

  if (!items.length) {
    ctx.fillStyle = '#8398B7'
    ctx.font = '20px Arial'
    ctx.fillText('За выбранный период данных нет.', x, y + 12)
  }
}

function drawDaily(
  ctx: CanvasRenderingContext2D,
  points: DailyPoint[],
  x: number,
  y: number,
  width: number,
  height: number
) {
  const max = Math.max(1, ...points.flatMap((p) => [p.created, p.resolved]))

  ctx.strokeStyle = '#20314B'
  ctx.lineWidth = 2
  for (let line = 0; line < 4; line++) {
    const py = y + (line * height) / 3
    ctx.beginPath()
    ctx.moveTo(x, py)
    ctx.lineTo(x + width, py)
    ctx.stroke()
  }

  const drawLine = (key: 'created' | 'resolved', color: string) => {
    if (!points.length) return
    ctx.strokeStyle = color
    ctx.lineWidth = 5
    ctx.beginPath()
    points.forEach((point, index) => {
      const px = x + (points.length <= 1 ? 0 : (index * width) / (points.length - 1))
      const py = y + height - (Number(point[key] || 0) / max) * height
      if (index === 0) ctx.moveTo(px, py)
      else ctx.lineTo(px, py)
    })
    ctx.stroke()
  }

  drawLine('created', '#43CDE5')
  drawLine('resolved', '#A18AF0')

  ctx.fillStyle = '#8398B7'
  ctx.font = '15px Arial'
  const labels = [points[0], points[Math.floor(points.length / 2)], points.at(-1)]
  labels.forEach((point, index) => {
    if (point) ctx.fillText(point.date, x + index * (width / 2), y + height + 26)
  })

  ctx.fillStyle = '#43CDE5'
  ctx.fillRect(x, y + height + 43, 16, 5)
  ctx.fillStyle = '#91A4C2'
  ctx.font = '15px Arial'
  ctx.fillText('Создано', x + 24, y + height + 49)

  ctx.fillStyle = '#A18AF0'
  ctx.fillRect(x + 135, y + height + 43, 16, 5)
  ctx.fillStyle = '#91A4C2'
  ctx.fillText('Решение принято', x + 159, y + height + 49)

  if (!points.length) {
    ctx.fillStyle = '#8398B7'
    ctx.font = '20px Arial'
    ctx.fillText('За выбранный период данных нет.', x, y + 32)
  }
}

function exportAnalyticsPNG(data: AnalyticsData, section: string = 'all') {
  const heights: Record<string, number> = {
    summary: 360,
    daily: 350,
    status: 310,
    types: 270,
    efficiency: 230,
  }

  const sections = section === 'all' ? Object.keys(heights) : [section]
  const width = 1600
  const headerHeight = 150
  const footerHeight = 90
  const height =
    headerHeight + sections.reduce((sum, key) => sum + (heights[key] || 260) + 22, 0) + footerHeight

  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  ctx.fillStyle = '#070E1D'
  ctx.fillRect(0, 0, width, height)

  ctx.fillStyle = '#EAF2FF'
  ctx.font = 'bold 42px Arial'
  ctx.fillText('FutureVoice · Аналитика обращений', 64, 70)

  ctx.fillStyle = '#91A4C2'
  ctx.font = '21px Arial'
  ctx.fillText(`${data.from} — ${data.to}`, 66, 110)

  let top = headerHeight

  sections.forEach((key) => {
    const panelHeight = heights[key] || 260
    ctx.fillStyle = '#0E1A30'
    roundedRect(ctx, 55, top, 1490, panelHeight, 20)

    const x = 90
    const y = top + 55

    if (key === 'summary') {
      drawTitle(ctx, 'Ключевые показатели', 'Количество обращений за выбранный период', x, y)
      const cards: [string, number][] = [
        ['Всего', data.total_requests],
        ['Новые', data.open_requests],
        ['В работе', data.in_progress_requests],
        ['Ожидают ответа', data.waiting_for_user_requests || 0],
        ['Завершены', data.closed_requests],
        ['Отклонены', data.rejected_requests || 0],
      ]
      const cardWidth = 450
      cards.forEach(([label, value], index) => {
        const column = index % 3
        const row = Math.floor(index / 3)
        const cardX = x + column * 475
        const cardY = top + 112 + row * 105
        ctx.fillStyle = '#15243C'
        roundedRect(ctx, cardX, cardY, cardWidth, 85, 12)
        ctx.fillStyle = '#91A4C2'
        ctx.font = '17px Arial'
        ctx.fillText(label, cardX + 18, cardY + 29)
        ctx.fillStyle = '#EAF2FF'
        ctx.font = 'bold 34px Arial'
        ctx.fillText(String(value ?? 0), cardX + 18, cardY + 68)
      })
    } else if (key === 'daily') {
      drawTitle(ctx, 'Динамика обращений', 'Созданные заявки и заявки с итоговым решением по дням', x, y)
      drawDaily(ctx, data.daily || [], x, top + 112, 1370, 170)
    } else if (key === 'status') {
      drawTitle(ctx, 'Статусы', 'Распределение по текущему состоянию обращений', x, y)
      drawDistribution(ctx, data.by_status || [], STATUS_LABELS, STATUS_COLORS, x, top + 112, 1370, 34)
    } else if (key === 'types') {
      drawTitle(ctx, 'Типы обращений', 'Распределение по категориям', x, y)
      drawDistribution(ctx, data.by_type || [], TYPE_LABELS, {}, x, top + 112, 1370, 46)
    } else if (key === 'efficiency') {
      drawTitle(
        ctx,
        'Эффективность обработки',
        'По обращениям, завершённым или отклонённым в выбранном периоде',
        x,
        y
      )
      ctx.fillStyle = '#EAF2FF'
      ctx.font = 'bold 48px Arial'
      ctx.fillText(
        data.average_resolution_hours == null
          ? '—'
          : `${Number(data.average_resolution_hours).toFixed(1)} ч`,
        x,
        top + 150
      )
      ctx.fillStyle = '#91A4C2'
      ctx.font = '20px Arial'
      ctx.fillText('Среднее время обработки', x, top + 184)

      ctx.fillStyle = '#43CDE5'
      ctx.font = 'bold 36px Arial'
      ctx.fillText(`${Number(data.resolution_rate || 0).toFixed(1)}%`, x + 620, top + 150)
      ctx.fillStyle = '#91A4C2'
      ctx.font = '20px Arial'
      ctx.fillText('Доля обращений с итоговым решением', x + 620, top + 184)
    }

    top += panelHeight + 22
  })

  ctx.fillStyle = '#91A4C2'
  ctx.font = '16px Arial'
  ctx.fillText(
    'Данные агрегированы. Персональные данные и тексты обращений не включены.',
    64,
    height - 52
  )
  ctx.fillText('FutureVoice · Школьное самоуправление', 64, height - 26)

  canvas.toBlob((blob) => {
    if (!blob) return
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `futurevoice-${section}-${data.from}-${data.to}.png`
    link.click()
    window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  }, 'image/png')
}

function ExportButton({ data, section }: { data: AnalyticsData; section: string }) {
  return (
    <button
      className="small-export-button"
      onClick={() => exportAnalyticsPNG(data, section)}
      title="Скачать этот раздел в PNG"
    >
      ↓ PNG
    </button>
  )
}

export default function DashboardPage() {
  const [range, setRange] = useState(30)
  const [reloadKey, setReloadKey] = useState(0)
  const [data, setData] = useState<AnalyticsData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const dates = useMemo(() => {
    const to = new Date()
    const from = new Date()
    from.setDate(to.getDate() - range + 1)
    const iso = (d: Date) =>
      `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    return { from: iso(from), to: iso(to) }
  }, [range])

  useEffect(() => {
    let active = true

    async function fetchOverview() {
      try {
        const result = await requestsApi.analytics(dates.from, dates.to)
        if (!active) return
        setData(result)
        setError('')
      } catch (err: any) {
        if (active) setError(err.message || 'Не удалось загрузить аналитику')
      } finally {
        if (active) setLoading(false)
      }
    }

    void fetchOverview()
    return () => {
      active = false
    }
  }, [dates.from, dates.to, reloadKey])

  const refresh = () => {
    setLoading(true)
    setReloadKey((v) => v + 1)
  }

  return (
    <div className="page-content">
      <Topbar
        title="Обзор"
        description="Сводка по обращениям и результатам работы школьной команды."
        right={
          <div className="toolbar">
            <select
              value={range}
              onChange={(e) => {
                setLoading(true)
                setRange(Number(e.target.value))
              }}
              aria-label="Период аналитики"
            >
              <option value={7}>7 дней</option>
              <option value={30}>30 дней</option>
              <option value={90}>90 дней</option>
              <option value={180}>180 дней</option>
              <option value={365}>Год</option>
            </select>
            <button className="secondary-button" disabled={loading} onClick={refresh}>
              ↻ Обновить
            </button>
            <button
              className="primary-button"
              disabled={!data || loading}
              onClick={() => data && exportAnalyticsPNG(data)}
            >
              ↓ Экспорт всей аналитики
            </button>
          </div>
        }
      />

      <ErrorBanner error={error} onClose={() => setError('')} />

      {loading && !data ? (
        <Loading />
      ) : data ? (
        <>
          <div className="analytics-section-heading">
            <div>
              <h2>Ключевые показатели</h2>
              <p>
                {data.from} — {data.to}
              </p>
            </div>
            <ExportButton data={data} section="summary" />
          </div>

          <div className="metric-grid">
            <MetricCard
              label="Поступило обращений"
              value={data.total_requests}
              note="Все типы"
              icon="＋"
              tone="violet"
            />
            <MetricCard
              label="Новые"
              value={data.open_requests}
              note="Ожидают рассмотрения"
              icon="◷"
              tone="amber"
            />
            <MetricCard
              label="В работе"
              value={data.in_progress_requests}
              note="Обрабатываются командой"
              icon="↗"
              tone="blue"
            />
            <MetricCard
              label="Ожидают ответа"
              value={data.waiting_for_user_requests || 0}
              note="Нужна информация от автора"
              icon="…"
              tone="violet"
            />
            <MetricCard
              label="Завершены"
              value={data.closed_requests}
              note="Работа закончена"
              icon="✓"
              tone="green"
            />
            <MetricCard
              label="Отклонены"
              value={data.rejected_requests || 0}
              note="С указанием причины"
              icon="×"
              tone="rose"
            />
          </div>

          {loading && <div className="muted small-space">Обновляем показатели…</div>}

          <div className="dashboard-grid">
            <section className="panel chart-panel panel-wide">
              <div className="panel-heading">
                <div>
                  <h2>Динамика обращений</h2>
                  <p>Созданные обращения и обращения с итоговым решением по дням</p>
                </div>
                <div className="panel-actions">
                  <Badge tone="blue">Динамика</Badge>
                  <ExportButton data={data} section="daily" />
                </div>
              </div>
              {data.daily?.length ? (
                <SvgLineChart points={data.daily} />
              ) : (
                <Empty title="Пока нет данных" description="За выбранный период заявок не поступало." />
              )}
            </section>

            <section className="panel chart-panel">
              <div className="panel-heading">
                <div>
                  <h2>Статусы</h2>
                  <p>Текущее состояние заявок периода</p>
                </div>
                <ExportButton data={data} section="status" />
              </div>
              <StatusDonut data={data.by_status || []} />
            </section>

            <section className="panel chart-panel">
              <div className="panel-heading">
                <div>
                  <h2>Типы обращений</h2>
                  <p>Распределение по категориям</p>
                </div>
                <ExportButton data={data} section="types" />
              </div>
              <TypeBars items={data.by_type || []} />
            </section>

            <section className="panel insight-panel">
              <div className="panel-heading">
                <div>
                  <h2>Эффективность обработки</h2>
                  <p>На основе истории изменения статусов</p>
                </div>
                <ExportButton data={data} section="efficiency" />
              </div>
              <div className="efficiency-metrics">
                <div>
                  <div className="resolution-value">
                    {data.average_resolution_hours == null
                      ? '—'
                      : `${Number(data.average_resolution_hours).toFixed(1)} ч`}
                  </div>
                  <div className="resolution-label">Среднее время обработки</div>
                </div>
                <div>
                  <div className="resolution-value">
                    {Number(data.resolution_rate || 0).toFixed(1)}%
                  </div>
                  <div className="resolution-label">Доля обращений с итоговым решением</div>
                </div>
              </div>
              <div className="insight-footnote">
                Учитываются обращения, которые были завершены или отклонены.
              </div>
            </section>
          </div>

          <div className="privacy-note">
            <span>ⓘ</span> Показатели агрегированы. Имена учеников, Telegram ID и тексты обращений не
            включаются в PNG-экспорт.
          </div>
        </>
      ) : (
        <Empty
          title="Не удалось получить аналитику"
          description="Проверьте соединение с backend и попробуйте обновить данные."
        />
      )}
    </div>
  )
}