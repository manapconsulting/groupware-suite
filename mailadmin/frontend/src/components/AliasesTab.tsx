import { useState, useEffect } from 'react';
import { aliases, domains } from '../api';
import type { Alias, Domain } from '../api';

function AliasesTab() {
  const [aliasList, setAliasList] = useState<Alias[]>([]);
  const [domainList, setDomainList] = useState<Domain[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [formData, setFormData] = useState({
    domain_id: 0,
    source: '',
    destination: '',
  });

  const fetchData = async () => {
    try {
      const [aliasesRes, domainsRes] = await Promise.all([
        aliases.list(),
        domains.list(),
      ]);
      if (aliasesRes.data.success) {
        setAliasList(aliasesRes.data.data || []);
      }
      if (domainsRes.data.success) {
        setDomainList(domainsRes.data.data || []);
        if (domainsRes.data.data && domainsRes.data.data.length > 0) {
          setFormData((prev) => ({ ...prev, domain_id: domainsRes.data.data![0].id }));
        }
      }
    } catch (err) {
      setError('Veriler alınamadı');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    try {
      const response = await aliases.create(formData as Alias);
      if (response.data.success) {
        setShowForm(false);
        setFormData({ domain_id: domainList[0]?.id || 0, source: '', destination: '' });
        fetchData();
      } else {
        setError(response.data.message || 'Oluşturulamadı');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Oluşturulamadı');
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Bu aliası silmek istediğinize emin misiniz?')) return;

    try {
      const response = await aliases.delete(id);
      if (response.data.success) {
        fetchData();
      } else {
        setError(response.data.message || 'Silinemedi');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Silinemedi');
    }
  };

  if (loading) return <div className="loading">Yükleniyor...</div>;

  return (
    <div className="tab-panel">
      <h2>Aliaslar (Yönlendirmeler)</h2>

      {error && <div className="error-message">{error}</div>}

      <button onClick={() => setShowForm(!showForm)} className="add-btn">
        {showForm ? 'İptal' : 'Yeni Alias'}
      </button>

      {showForm && (
        <form onSubmit={handleCreate} className="add-form vertical">
          <div className="form-group">
            <label>Domain</label>
            <select
              value={formData.domain_id}
              onChange={(e) =>
                setFormData({ ...formData, domain_id: parseInt(e.target.value) })
              }
            >
              {domainList.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                </option>
              ))}
            </select>
          </div>
          <div className="form-group">
            <label>Kaynak (From)</label>
            <input
              type="text"
              value={formData.source}
              onChange={(e) => setFormData({ ...formData, source: e.target.value })}
              placeholder="alias@domain.com"
              required
            />
          </div>
          <div className="form-group">
            <label>Hedef (To)</label>
            <input
              type="text"
              value={formData.destination}
              onChange={(e) =>
                setFormData({ ...formData, destination: e.target.value })
              }
              placeholder="hedef@domain.com"
              required
            />
          </div>
          <button type="submit">Oluştur</button>
        </form>
      )}

      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Kaynak</th>
            <th>Hedef</th>
            <th>Domain</th>
            <th>İşlem</th>
          </tr>
        </thead>
        <tbody>
          {aliasList.map((alias) => (
            <tr key={alias.id}>
              <td>{alias.id}</td>
              <td>{alias.source}</td>
              <td>{alias.destination}</td>
              <td>{alias.domain}</td>
              <td>
                <button
                  onClick={() => handleDelete(alias.id)}
                  className="delete-btn"
                >
                  Sil
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default AliasesTab;
