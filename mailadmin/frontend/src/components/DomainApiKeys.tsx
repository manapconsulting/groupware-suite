import { useState, useEffect } from 'react';
import { apiKeys } from '../api';
import type { Domain, DomainAPIKey } from '../api';

// Splits the free-text sources field (comma or newline separated) into entries.
const parseSources = (s: string) =>
  s.split(/[\n,]/).map((x) => x.trim()).filter(Boolean);

function DomainApiKeys({ domain }: { domain: Domain }) {
  const [keys, setKeys] = useState<DomainAPIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState('');
  const [sources, setSources] = useState('');
  const [editingId, setEditingId] = useState<number | null>(null);
  // Plaintext of a freshly-created key: shown once, never retrievable again.
  const [newKey, setNewKey] = useState<string | null>(null);

  const fetchKeys = async () => {
    setLoading(true);
    try {
      const res = await apiKeys.list(domain.id);
      if (res.data.success) setKeys(res.data.data || []);
    } catch {
      setError('API anahtarlari alinamadi');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setNewKey(null);
    setError('');
    resetForm();
    fetchKeys();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [domain.id]);

  const resetForm = () => {
    setName('');
    setSources('');
    setEditingId(null);
    setShowForm(false);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    const list = parseSources(sources);
    if (!name.trim() || list.length === 0) {
      setError('Isim ve en az bir izinli kaynak (IP, CIDR veya FQDN) gerekli');
      return;
    }
    try {
      if (editingId) {
        const res = await apiKeys.update(domain.id, editingId, { name: name.trim(), allowed_sources: list });
        if (res.data.success) {
          resetForm();
          fetchKeys();
        } else {
          setError(res.data.message || 'Guncellenemedi');
        }
      } else {
        const res = await apiKeys.create(domain.id, { name: name.trim(), allowed_sources: list });
        if (res.data.success && res.data.data) {
          setNewKey(res.data.data.key);
          resetForm();
          fetchKeys();
        } else {
          setError(res.data.message || 'Olusturulamadi');
        }
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Islem basarisiz');
    }
  };

  const startEdit = (k: DomainAPIKey) => {
    setEditingId(k.id);
    setName(k.name);
    setSources(k.allowed_sources.join('\n'));
    setShowForm(true);
    setNewKey(null);
  };

  const toggleEnabled = async (k: DomainAPIKey) => {
    try {
      await apiKeys.update(domain.id, k.id, {
        name: k.name,
        allowed_sources: k.allowed_sources,
        enabled: !k.enabled,
      });
      fetchKeys();
    } catch (err: any) {
      setError(err.response?.data?.message || 'Guncellenemedi');
    }
  };

  const handleDelete = async (k: DomainAPIKey) => {
    if (!confirm(`"${k.name}" anahtarini iptal etmek istediginize emin misiniz? Bu islem geri alinamaz.`)) return;
    try {
      await apiKeys.delete(domain.id, k.id);
      fetchKeys();
    } catch (err: any) {
      setError(err.response?.data?.message || 'Silinemedi');
    }
  };

  return (
    <div className="api-keys-panel">
      <div className="api-keys-intro">
        <p>
          Bu anahtarlar yalnizca <strong>{domain.name}</strong> icin posta adresi olusturmayi ve
          yonetmeyi saglar; sadece asagida belirttiginiz IP / CIDR / FQDN kaynaklardan kullanilabilir.
          Istekte <code>X-API-Key</code> basligi ile gonderilir.
        </p>
      </div>

      {error && <div className="error-message">{error}</div>}

      {newKey && (
        <div className="api-key-created">
          <strong>Yeni anahtar olusturuldu — bu deger yalnizca bir kez gosterilir:</strong>
          <div className="api-key-secret">
            <code>{newKey}</code>
            <button
              className="copy-btn"
              onClick={() => {
                navigator.clipboard.writeText(newKey);
                alert('Kopyalandi!');
              }}
            >
              Kopyala
            </button>
          </div>
          <button className="add-btn small" onClick={() => setNewKey(null)}>Tamam</button>
        </div>
      )}

      <div className="api-keys-header">
        <h4>API Anahtarlari</h4>
        {!showForm && (
          <button className="add-btn small" onClick={() => { setShowForm(true); setNewKey(null); }}>
            + Yeni Anahtar
          </button>
        )}
      </div>

      {showForm && (
        <form onSubmit={handleSubmit} className="api-key-form">
          <label>
            Anahtar adi
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="orn. vendisens provisioning"
              autoFocus
            />
          </label>
          <label>
            Izinli kaynaklar (her satira bir IP, CIDR veya FQDN)
            <textarea
              value={sources}
              onChange={(e) => setSources(e.target.value)}
              placeholder={'203.0.113.10\n10.0.0.0/24\nprovision.example.com'}
              rows={4}
            />
          </label>
          <div className="form-actions">
            <button type="submit">{editingId ? 'Kaydet' : 'Olustur'}</button>
            <button type="button" className="delete-btn" onClick={resetForm}>Iptal</button>
          </div>
        </form>
      )}

      {loading ? (
        <div className="loading">Yukleniyor...</div>
      ) : keys.length === 0 ? (
        <p className="empty-state">Bu domain icin henuz API anahtari yok.</p>
      ) : (
        <table className="api-keys-table">
          <thead>
            <tr>
              <th>Ad</th>
              <th>Anahtar</th>
              <th>Izinli Kaynaklar</th>
              <th>Durum</th>
              <th>Son Kullanim</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {keys.map((k) => (
              <tr key={k.id} className={k.enabled ? '' : 'disabled-row'}>
                <td>{k.name}</td>
                <td><code>{k.key_prefix}…</code></td>
                <td>
                  {k.allowed_sources.map((s) => (
                    <span key={s} className="source-chip">{s}</span>
                  ))}
                </td>
                <td>{k.enabled ? 'Aktif' : 'Pasif'}</td>
                <td>{k.last_used_at ? k.last_used_at : '—'}</td>
                <td className="row-actions">
                  <button className="refresh-btn" onClick={() => startEdit(k)}>Duzenle</button>
                  <button className="refresh-btn" onClick={() => toggleEnabled(k)}>
                    {k.enabled ? 'Pasiflestir' : 'Aktiflestir'}
                  </button>
                  <button className="delete-btn" onClick={() => handleDelete(k)}>Iptal Et</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

export default DomainApiKeys;
