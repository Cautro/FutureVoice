import { useEffect, useState } from 'react'
import { requestsApi } from '../api/requests'
import { Badge, Empty, ErrorBanner, Loading, Topbar } from '../components/UI'

interface UserItem {
  id: number
  first_name?: string
  last_name?: string
  username?: string
  telegram_id: number
  role: string
  is_active: boolean
}

interface UsersResult {
  items: UserItem[]
  total: number
}

export default function UsersPage({ user }: { user: { id: number } }) {
  const [role, setRole] = useState('')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [reloadKey, setReloadKey] = useState(0)
  const [result, setResult] = useState<UsersResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [savingID, setSavingID] = useState<number | null>(null)

  useEffect(() => {
    let active = true

    async function fetchUsers() {
      try {
        const response = await requestsApi.users({
          role,
          q: query.trim(),
          page,
          limit: 20,
        })
        if (!active) return
        setResult(response)
        setError('')
      } catch (err: any) {
        if (active) setError(err.message || 'Не удалось загрузить пользователей')
      } finally {
        if (active) setLoading(false)
      }
    }

    void fetchUsers()
    return () => {
      active = false
    }
  }, [role, query, page, reloadKey])

  function refresh() {
    setLoading(true)
    setReloadKey((v) => v + 1)
  }

  async function changeRole(item: UserItem, nextRole: string) {
    setSavingID(item.id)
    setError('')
    try {
      await requestsApi.updateRole(item.id, nextRole)
      refresh()
    } catch (err: any) {
      setError(err.message || 'Не удалось изменить роль')
    } finally {
      setSavingID(null)
    }
  }

  async function changeActive(item: UserItem) {
    setSavingID(item.id)
    setError('')
    try {
      await requestsApi.updateActive(item.id, !item.is_active)
      refresh()
    } catch (err: any) {
      setError(err.message || 'Не удалось изменить доступ')
    } finally {
      setSavingID(null)
    }
  }

  return (
    <div className="page-content">
      <Topbar title="Пользователи" description="Управление ролями и доступом к панели модерации." />

      <ErrorBanner error={error} onClose={() => setError('')} />

      <section className="panel">
        <div className="filter-bar">
          <div className="search-field">
            <span>⌕</span>
            <input
              value={query}
              onChange={(e) => {
                setLoading(true)
                setQuery(e.target.value)
                setPage(1)
              }}
              placeholder="Поиск по имени, username или Telegram ID"
            />
          </div>
          <select
            value={role}
            onChange={(e) => {
              setLoading(true)
              setRole(e.target.value)
              setPage(1)
            }}
          >
            <option value="">Все роли</option>
            <option value="student">Ученики</option>
            <option value="moderator">Модераторы</option>
            <option value="admin">Администраторы</option>
          </select>
          <button className="secondary-button" disabled={loading} onClick={refresh}>
            ↻
          </button>
        </div>

        {loading && !result ? (
          <Loading />
        ) : result?.items?.length ? (
          <>
            <div className="user-table">
              <div className="user-table-head">
                <span>Пользователь</span>
                <span>Telegram ID</span>
                <span>Роль</span>
                <span>Доступ</span>
                <span>Управление</span>
              </div>
              {result.items.map((item) => (
                <div className="user-row" key={item.id}>
                  <span className="user-name">
                    <span className="avatar small-avatar">
                      {(item.first_name || item.username || 'U').slice(0, 1).toUpperCase()}
                    </span>
                    <span>
                      <strong>
                        {[item.first_name, item.last_name].filter(Boolean).join(' ') || 'Без имени'}
                      </strong>
                      <small>
                        {item.username ? `@${item.username}` : `ID записи ${item.id}`}
                      </small>
                    </span>
                  </span>
                  <span className="mono-cell">
                    {item.telegram_id > 0 ? item.telegram_id : 'Не привязан'}
                  </span>
                  <span>
                    <select
                      value={item.role}
                      disabled={savingID === item.id || item.id === user.id}
                      onChange={(e) => changeRole(item, e.target.value)}
                    >
                      <option value="student">Ученик</option>
                      <option value="moderator">Модератор</option>
                      <option value="admin">Администратор</option>
                    </select>
                  </span>
                  <span>
                    <Badge tone={item.is_active ? 'green' : 'red'}>
                      {item.is_active ? 'Активен' : 'Отключён'}
                    </Badge>
                  </span>
                  <span>
                    <button
                      className="text-button"
                      disabled={savingID === item.id || item.id === user.id}
                      onClick={() => changeActive(item)}
                    >
                      {item.is_active ? 'Отключить' : 'Включить'}
                    </button>
                  </span>
                </div>
              ))}
            </div>

            <div className="pagination">
              <button
                className="secondary-button"
                disabled={page <= 1 || loading}
                onClick={() => {
                  setLoading(true)
                  setPage((v) => v - 1)
                }}
              >
                ← Назад
              </button>
              <span>
                {result.total} пользователей · страница {page}
              </span>
              <button
                className="secondary-button"
                disabled={page * 20 >= result.total || loading}
                onClick={() => {
                  setLoading(true)
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
          <Empty
            title="Пользователи не найдены"
            description="Попробуйте изменить фильтр или поисковый запрос."
          />
        )}
      </section>

      <div className="privacy-note">
        <span>!</span> Система не позволяет отключить или понизить роль последнего активного
        администратора.
      </div>
    </div>
  )
}