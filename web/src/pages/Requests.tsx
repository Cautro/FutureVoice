import { useCallback, useEffect, useRef, useState } from 'react'
import { requestsApi } from '../api/requests'
import { Badge, Empty, ErrorBanner, Loading, Topbar } from '../components/UI'
import { STATUS, TYPE } from '../utils/constants'
import { formatDate, fullName, statusLabel } from '../utils/format'

interface User {
  id: number
  role: string
}

interface RequestItem {
  id: number
  theme: string
  request_type: string
  status: string
  created_at: string
  first_name?: string
  last_name?: string
  username?: string
}

interface RequestsResult {
  items: RequestItem[]
  total: number
  page: number
}

export default function RequestsPage({ user }: { user: User }) {
  const [status, setStatus] = useState('')
  const [type, setType] = useState('')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [reloadKey, setReloadKey] = useState(0)
  const [result, setResult] = useState<RequestsResult | null>(null)
  const [selectedID, setSelectedID] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true

    async function fetchRequests() {
      try {
        const response = await requestsApi.list({
          status,
          type,
          q: query.trim(),
          page,
          limit: 20,
        })
        if (!active) return
        setResult(response)
        setError('')
      } catch (err: any) {
        if (active) setError(err.message || 'Не удалось загрузить обращения')
      } finally {
        if (active) setLoading(false)
      }
    }

    void fetchRequests()
    return () => {
      active = false
    }
  }, [status, type, query, page, reloadKey])

  const refresh = useCallback(() => {
    setLoading(true)
    setReloadKey((v) => v + 1)
  }, [])

  function updateFilter(setter: (v: any) => void, value: any) {
    setLoading(true)
    setSelectedID(null)
    setPage(1)
    setter(value)
  }

  return (
    <div className="page-content">
      <Topbar
        title="Заявки"
        description="Очередь обращений: фильтруйте, назначайте ответственных и фиксируйте результат."
      />

      <ErrorBanner error={error} onClose={() => setError('')} />

      <section className="panel">
        <div className="filter-bar">
          <div className="search-field">
            <span>⌕</span>
            <input
              value={query}
              onChange={(e) => updateFilter(setQuery, e.target.value)}
              placeholder="Поиск по теме, тексту или номеру"
            />
          </div>
          <select value={status} onChange={(e) => updateFilter(setStatus, e.target.value)}>
            <option value="">Все статусы</option>
            <option value="open">Новые</option>
            <option value="in_progress">В работе</option>
            <option value="waiting_for_user">Ожидают ответа</option>
            <option value="closed">Завершённые</option>
            <option value="rejected">Отклонённые</option>
          </select>
          <select value={type} onChange={(e) => updateFilter(setType, e.target.value)}>
            <option value="">Все типы</option>
            <option value="complaint">Жалобы</option>
            <option value="offer">Предложения</option>
            <option value="3dPrintOffer">3D-печать</option>
          </select>
          <button className="secondary-button" disabled={loading} onClick={refresh}>
            ↻
          </button>
        </div>

        <div className="table-meta">
          <span>{result?.total ?? 0} обращений</span>
          <span>
            Страница {result?.page ?? page} из {Math.max(1, Math.ceil((result?.total || 0) / 20))}
          </span>
        </div>

        {loading && !result ? (
          <Loading />
        ) : result?.items?.length ? (
          <>
            <div className="request-table">
              <div className="request-table-head">
                <span>Обращение</span>
                <span>Автор</span>
                <span>Статус</span>
                <span>Дата</span>
              </div>
              {result.items.map((item) => (
                <button
                  key={item.id}
                  className={`request-row ${selectedID === item.id ? 'selected' : ''}`}
                  onClick={() => setSelectedID(item.id)}
                >
                  <span className="request-main-cell">
                    <strong>
                      #{item.id} · {item.theme}
                    </strong>
                    <small>{TYPE[item.request_type] || item.request_type}</small>
                  </span>
                  <span className="request-author-cell">{fullName(item)}</span>
                  <span>
                    <Badge tone={STATUS[item.status]?.tone}>{statusLabel(item.status)}</Badge>
                  </span>
                  <span className="request-date-cell">{formatDate(item.created_at)}</span>
                </button>
              ))}
            </div>

            <div className="pagination">
              <button
                className="secondary-button"
                disabled={page <= 1 || loading}
                onClick={() => {
                  setLoading(true)
                  setSelectedID(null)
                  setPage((v) => v - 1)
                }}
              >
                ← Назад
              </button>
              <button
                className="secondary-button"
                disabled={page * 20 >= (result.total || 0) || loading}
                onClick={() => {
                  setLoading(true)
                  setSelectedID(null)
                  setPage((v) => v + 1)
                }}
              >
                Далее →
              </button>
            </div>
          </>
        ) : loading ? (
          <Loading />
        ) : (
          <Empty title="Заявки не найдены" description="Попробуйте изменить фильтры или поисковый запрос." />
        )}
      </section>

      {selectedID && (
        <RequestDetails
          id={selectedID}
          user={user}
          onClose={() => setSelectedID(null)}
          onChanged={refresh}
        />
      )}
    </div>
  )
}

// --- RequestDetails (модал) ---

function RequestDetails({
  id,
  user,
  onClose,
  onChanged,
}: {
  id: number
  user: User
  onClose: () => void
  onChanged: () => void
}) {
  const [details, setDetails] = useState<any>(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [nextStatus, setNextStatus] = useState('')
  const [comment, setComment] = useState('')
  const [commentVisibility, setCommentVisibility] = useState<'student' | 'internal'>('student')
  const [statusReason, setStatusReason] = useState('')
  const lastPersistedStatus = useRef('')
  const isAdmin = user.role === 'admin'

  useEffect(() => {
    setDetails(null)
    setNextStatus('')
    setStatusReason('')
    setComment('')
    setCommentVisibility('student')
    setError('')
    lastPersistedStatus.current = ''
    setLoading(true)
  }, [id])

  useEffect(() => {
    const timer = window.setInterval(() => setReloadKey((v) => v + 1), 5000)
    return () => window.clearInterval(timer)
  }, [id])

  useEffect(() => {
    let active = true

    async function fetchDetails() {
      try {
        const response = await requestsApi.get(id)
        if (!active) return
        setDetails(response)
        const prev = lastPersistedStatus.current
        setNextStatus((current) => {
          if (!current || current === prev) return response.request.status
          return current
        })
        lastPersistedStatus.current = response.request.status
        setError('')
      } catch (err: any) {
        if (active) setError(err.message || 'Не удалось загрузить заявку')
      } finally {
        if (active) setLoading(false)
      }
    }

    void fetchDetails()
    return () => {
      active = false
    }
  }, [id, reloadKey])

  function refreshDetails() {
    setLoading(true)
    setReloadKey((v) => v + 1)
  }

  async function updateStatus() {
    setSaving(true)
    setError('')
    try {
      await requestsApi.updateStatus(id, nextStatus, statusReason.trim())
      setStatusReason('')
      refreshDetails()
      onChanged()
    } catch (err: any) {
      setError(err.message || 'Не удалось изменить статус')
    } finally {
      setSaving(false)
    }
  }

  async function assignToMe() {
    setSaving(true)
    setError('')
    try {
      await requestsApi.assign(id, user.id)
      refreshDetails()
      onChanged()
    } catch (err: any) {
      setError(err.message || 'Не удалось назначить заявку')
    } finally {
      setSaving(false)
    }
  }

  async function addComment(e: React.FormEvent) {
    e.preventDefault()
    if (!comment.trim()) return
    setSaving(true)
    setError('')
    try {
      await requestsApi.addMessage(id, comment.trim(), commentVisibility)
      setComment('')
      refreshDetails()
    } catch (err: any) {
      setError(err.message || 'Не удалось добавить комментарий')
    } finally {
      setSaving(false)
    }
  }

  async function remove() {
    if (!window.confirm(`Удалить заявку #${id}? Она будет скрыта из очереди, но останется в аудите.`)) return
    setSaving(true)
    setError('')
    try {
      await requestsApi.remove(id)
      onClose()
      onChanged()
    } catch (err: any) {
      setError(err.message || 'Не удалось удалить заявку')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div
      className="modal-backdrop"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <section className="details-modal" role="dialog" aria-modal="true" aria-label={`Заявка ${id}`}>
        <div className="details-header">
          <div>
            <span className="breadcrumbs">ОБРАЩЕНИЯ / #{id}</span>
            <h2>Карточка заявки</h2>
          </div>
          <button className="icon-button close-button" onClick={onClose} aria-label="Закрыть">
            ×
          </button>
        </div>

        <ErrorBanner error={error} onClose={() => setError('')} />

        {loading && !details ? (
          <Loading />
        ) : details ? (
          <div className="details-body">
            <div className="details-main">
              <div className="detail-title-row">
                <h2>{details.request.theme}</h2>
                <Badge tone={STATUS[details.request.status]?.tone}>
                  {statusLabel(details.request.status)}
                </Badge>
              </div>
              <div className="detail-meta">
                <Badge>{TYPE[details.request.request_type] || details.request.request_type}</Badge>
                <span>Создана {formatDate(details.request.created_at)}</span>
              </div>

              <div className="detail-section">
                <label>Автор</label>
                <strong>{fullName(details.request)}</strong>
              </div>

              <div className="detail-section">
                <label>Описание</label>
                <p className="request-description">{details.request.description}</p>
              </div>

              <div className="detail-section">
                <label>Переписка с пользователем и заметки</label>
                <div className="conversation-list">
                  {details.comments?.length > 0 ? (
                    details.comments.map((item: any) => {
                      const isInternal = item.visibility === 'internal'
                      const roleName =
                        item.author_role === 'student'
                          ? 'Ученик'
                          : item.author_role === 'admin'
                            ? 'Администратор'
                            : item.author_role === 'moderator'
                              ? 'Модератор'
                              : item.author_role

                      return (
                        <div
                          className={`comment-item ${isInternal ? 'internal' : 'public'}`}
                          key={item.id}
                        >
                          <div className="comment-meta">
                            <strong>{item.author_name || roleName}</strong>
                            <span>
                              {isInternal ? 'Внутренняя заметка' : roleName} ·{' '}
                              {formatDate(item.created_at)}
                            </span>
                          </div>
                          <p>{item.body}</p>
                        </div>
                      )
                    })
                  ) : (
                    <p className="muted small-space">Сообщений пока нет.</p>
                  )}
                </div>

                <form className="comment-form" onSubmit={addComment}>
                  <select
                    value={commentVisibility}
                    onChange={(e) => setCommentVisibility(e.target.value as any)}
                    aria-label="Видимость сообщения"
                  >
                    <option value="student">Ответить пользователю в Telegram</option>
                    <option value="internal">Добавить внутреннюю заметку</option>
                  </select>
                  <textarea
                    value={comment}
                    onChange={(e) => setComment(e.target.value)}
                    maxLength={commentVisibility === 'student' ? 3700 : 4000}
                    placeholder={
                      commentVisibility === 'student'
                        ? 'Сообщение будет отправлено пользователю в Telegram…'
                        : 'Комментарий виден только команде модерации…'
                    }
                    rows={3}
                  />
                  <div className="form-footer">
                    <small>
                      {comment.length}/{commentVisibility === 'student' ? 3700 : 4000}
                    </small>
                    <button
                      className="secondary-button"
                      disabled={saving || !comment.trim()}
                    >
                      {commentVisibility === 'student' ? 'Отправить ответ' : 'Добавить заметку'}
                    </button>
                  </div>
                </form>
              </div>

              <div className="detail-section">
                <label>История действий</label>
                <div className="timeline">
                  {details.events?.length ? (
                    details.events.map((event: any) => (
                      <div className="timeline-item" key={event.id}>
                        <span className="timeline-dot" />
                        <div>
                          <strong>{eventLabel(event)}</strong>
                          <small>
                            {event.actor_name} · {formatDate(event.created_at)}
                          </small>
                          {event.details && <p>{event.details}</p>}
                        </div>
                      </div>
                    ))
                  ) : (
                    <p className="muted">История пока пуста.</p>
                  )}
                </div>
              </div>
            </div>

            <aside className="details-actions">
              <div className="action-card">
                <h3>Обработка</h3>
                <label htmlFor="request-status">Статус</label>
                <select
                  id="request-status"
                  value={nextStatus}
                  onChange={(e) => {
                    setNextStatus(e.target.value)
                    setStatusReason('')
                  }}
                >
                  <option value="open">Новая</option>
                  <option value="in_progress">В работе</option>
                  <option value="waiting_for_user">Ожидает ответа пользователя</option>
                  <option value="closed">Завершена</option>
                  <option value="rejected">Отклонена</option>
                </select>

                {(nextStatus === 'rejected' || nextStatus === 'closed') && (
                  <div className="status-reason-field">
                    <label htmlFor="status-reason">
                      {nextStatus === 'rejected'
                        ? 'Причина отклонения *'
                        : 'Комментарий к завершению (необязательно)'}
                    </label>
                    <textarea
                      id="status-reason"
                      value={statusReason}
                      onChange={(e) => setStatusReason(e.target.value)}
                      maxLength={1000}
                      rows={3}
                      placeholder={
                        nextStatus === 'rejected'
                          ? 'Объясни пользователю, почему обращение отклонено…'
                          : 'Кратко опиши результат работы…'
                      }
                    />
                  </div>
                )}

                <button
                  className="primary-button full-width"
                  disabled={
                    saving ||
                    loading ||
                    nextStatus === details.request.status ||
                    (nextStatus === 'rejected' && statusReason.trim().length < 5)
                  }
                  onClick={updateStatus}
                >
                  Сохранить статус
                </button>

                <div className="action-divider" />

                <label>Ответственный</label>
                <div className="assignee-summary">
                  {details.request.assignee_first_name ||
                    details.request.assignee_username ||
                    'Не назначен'}
                </div>
                <button
                  className="secondary-button full-width"
                  disabled={saving || loading || details.request.assigned_to === user.id}
                  onClick={assignToMe}
                >
                  Назначить на себя
                </button>

                {isAdmin && (
                  <button
                    className="danger-button full-width"
                    disabled={saving || loading}
                    onClick={remove}
                  >
                    Удалить заявку
                  </button>
                )}
              </div>

              <div className="action-note">
                <strong>Доступ</strong>
                <p>
                  Ответы с пометкой «пользователю» отправляются в Telegram. Внутренние заметки видны
                  только команде. Изменения статуса автоматически отправляют уведомление
                  пользователю.
                </p>
              </div>
            </aside>
          </div>
        ) : (
          <Empty
            title="Заявка недоступна"
            description="Возможно, она была удалена или у вас нет доступа."
          />
        )}
      </section>
    </div>
  )
}

function eventLabel(event: any) {
  const labels: Record<string, string> = {
    created: 'Заявка создана',
    deleted: 'Заявка удалена',
    status_changed: `Статус: ${statusLabel(event.old_status)} → ${statusLabel(event.new_status)}`,
    assignee_changed: 'Изменён ответственный',
    internal_comment_added: 'Добавлена внутренняя заметка',
    staff_message_added: 'Отправлен ответ пользователю',
    student_message_added: 'Получено сообщение от пользователя',
  }
  return labels[event.event_type] || event.event_type
}