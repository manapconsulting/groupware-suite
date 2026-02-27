import { useState, useEffect } from 'react';
import { senderPermissions } from '../api';
import type { SenderPermission } from '../api';

function SenderPermissionsTab() {
  const [permList, setPermList] = useState<SenderPermission[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [formData, setFormData] = useState({
    send_as: '',
    login_user: '',
  });

  const fetchData = async () => {
    try {
      const response = await senderPermissions.list();
      if (response.data.success) {
        setPermList(response.data.data || []);
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
      const response = await senderPermissions.create(formData as SenderPermission);
      if (response.data.success) {
        setShowForm(false);
        setFormData({ send_as: '', login_user: '' });
        fetchData();
      } else {
        setError(response.data.message || 'Oluşturulamadı');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Oluşturulamadı');
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Bu izni silmek istediğinize emin misiniz?')) return;

    try {
      const response = await senderPermissions.delete(id);
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
      <h2>Gönderim İzinleri (Send-As)</h2>
      <p className="info-text">
        Bu bölümde kullanıcılara başka adresler adına mail gönderme izni verebilirsiniz.
      </p>

      {error && <div className="error-message">{error}</div>}

      <button onClick={() => setShowForm(!showForm)} className="add-btn">
        {showForm ? 'İptal' : 'Yeni İzin'}
      </button>

      {showForm && (
        <form onSubmit={handleCreate} className="add-form vertical">
          <div className="form-group">
            <label>Gönderilecek Adres (Send As)</label>
            <input
              type="email"
              value={formData.send_as}
              onChange={(e) => setFormData({ ...formData, send_as: e.target.value })}
              placeholder="grup@domain.com"
              required
            />
          </div>
          <div className="form-group">
            <label>Yetkili Kullanıcı (Login User)</label>
            <input
              type="email"
              value={formData.login_user}
              onChange={(e) =>
                setFormData({ ...formData, login_user: e.target.value })
              }
              placeholder="kullanici@domain.com"
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
            <th>Gönderilecek Adres</th>
            <th>Yetkili Kullanıcı</th>
            <th>İşlem</th>
          </tr>
        </thead>
        <tbody>
          {permList.map((perm) => (
            <tr key={perm.id}>
              <td>{perm.id}</td>
              <td>{perm.send_as}</td>
              <td>{perm.login_user}</td>
              <td>
                <button
                  onClick={() => handleDelete(perm.id)}
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

export default SenderPermissionsTab;
