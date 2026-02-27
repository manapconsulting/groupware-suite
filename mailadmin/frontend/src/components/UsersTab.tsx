import { useState, useEffect } from 'react';
import { users, domains } from '../api';
import type { User, Domain } from '../api';

function generatePassword(length = 10): string {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789';
  let password = '';
  for (let i = 0; i < length; i++) {
    password += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return password;
}

function UsersTab() {
  const [userList, setUserList] = useState<User[]>([]);
  const [domainList, setDomainList] = useState<Domain[]>([]);
  const [selectedDomain, setSelectedDomain] = useState<Domain | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [showPasswordForm, setShowPasswordForm] = useState<number | null>(null);
  const [newPassword, setNewPassword] = useState('');
  const [formData, setFormData] = useState({
    domain_id: 0,
    email: '',
    password: generatePassword(),
    notify_email: '',
  });

  const fetchData = async () => {
    try {
      const [usersRes, domainsRes] = await Promise.all([
        users.list(),
        domains.list(),
      ]);
      if (usersRes.data.success) {
        setUserList(usersRes.data.data || []);
      }
      if (domainsRes.data.success) {
        const domainData = domainsRes.data.data || [];
        setDomainList(domainData);
        if (domainData.length > 0 && !selectedDomain) {
          setSelectedDomain(domainData[0]);
          setFormData((prev) => ({ ...prev, domain_id: domainData[0].id }));
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

  // Domain seçildiğinde form domain_id'sini güncelle
  useEffect(() => {
    if (selectedDomain) {
      setFormData((prev) => ({ ...prev, domain_id: selectedDomain.id }));
    }
  }, [selectedDomain]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    try {
      const response = await users.create(formData as User);
      if (response.data.success) {
        setShowForm(false);
        setFormData({ domain_id: selectedDomain?.id || 0, email: '', password: generatePassword(), notify_email: '' });
        fetchData();
      } else {
        setError(response.data.message || 'Oluşturulamadı');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Oluşturulamadı');
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Bu kullanıcıyı silmek istediğinize emin misiniz?')) return;

    try {
      const response = await users.delete(id);
      if (response.data.success) {
        fetchData();
      } else {
        setError(response.data.message || 'Silinemedi');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Silinemedi');
    }
  };

  const handlePasswordChange = async (id: number) => {
    if (!newPassword.trim()) return;

    try {
      const response = await users.changePassword(id, newPassword);
      if (response.data.success) {
        setShowPasswordForm(null);
        setNewPassword('');
        alert('Şifre değiştirildi');
      } else {
        setError(response.data.message || 'Şifre değiştirilemedi');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Şifre değiştirilemedi');
    }
  };

  // Seçili domain'e göre filtrelenmiş kullanıcılar
  const filteredUsers = selectedDomain
    ? userList.filter((u) => u.domain_id === selectedDomain.id)
    : userList;

  // Her domain için kullanıcı sayısı
  const getUserCount = (domainId: number) => {
    return userList.filter((u) => u.domain_id === domainId).length;
  };

  if (loading) return <div className="loading">Yükleniyor...</div>;

  return (
    <div className="tab-panel">
      <h2>Kullanıcılar</h2>

      {error && <div className="error-message">{error}</div>}

      <div className="users-container">
        <div className="domains-sidebar">
          <h3>Domainler</h3>
          <ul className="domain-items">
            {domainList.map((domain) => (
              <li
                key={domain.id}
                className={selectedDomain?.id === domain.id ? 'active' : ''}
                onClick={() => setSelectedDomain(domain)}
              >
                <div className="domain-item-content">
                  <strong>{domain.name}</strong>
                  <span className="user-count">{getUserCount(domain.id)} kullanıcı</span>
                </div>
              </li>
            ))}
          </ul>
        </div>

        <div className="users-detail">
          {selectedDomain ? (
            <>
              <div className="users-header">
                <h3>{selectedDomain.name}</h3>
                <button onClick={() => setShowForm(!showForm)} className="add-btn small">
                  {showForm ? 'İptal' : 'Yeni Kullanıcı'}
                </button>
              </div>

              {showForm && (
                <form onSubmit={handleCreate} className="add-form vertical compact">
                  <div className="form-group">
                    <label>E-posta</label>
                    <div className="email-input-group">
                      <input
                        type="text"
                        value={formData.email.split('@')[0] || ''}
                        onChange={(e) => setFormData({ ...formData, email: e.target.value + '@' + selectedDomain.name })}
                        placeholder="kullanici"
                        required
                      />
                      <span className="email-domain">@{selectedDomain.name}</span>
                    </div>
                  </div>
                  <div className="form-group">
                    <label>Şifre</label>
                    <div className="password-input-group">
                      <input
                        type="text"
                        value={formData.password}
                        onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                        required
                      />
                      <button
                        type="button"
                        onClick={() => setFormData({ ...formData, password: generatePassword() })}
                        className="generate-btn"
                      >
                        Yenile
                      </button>
                    </div>
                  </div>
                  <div className="form-group">
                    <label>Bildirim E-postası (opsiyonel)</label>
                    <input
                      type="text"
                      autoComplete="off"
                      value={formData.notify_email}
                      onChange={(e) => setFormData({ ...formData, notify_email: e.target.value })}
                      placeholder="ornek@mail.com, diger@mail.com"
                      style={{ imeMode: 'disabled' }}
                    />
                    <small className="form-hint">Hesap bilgileri bu adrese gönderilir. Birden fazla adres için virgül kullanın.</small>
                  </div>
                  <button type="submit">Oluştur</button>
                </form>
              )}

              <div className="table-container">
                <table>
                  <thead>
                    <tr>
                      <th>E-posta</th>
                      <th>İşlem</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredUsers.length === 0 ? (
                      <tr>
                        <td colSpan={2} style={{ textAlign: 'center', color: '#6b7280' }}>
                          Bu domainde kullanıcı yok
                        </td>
                      </tr>
                    ) : (
                      filteredUsers.map((user) => (
                        <tr key={user.id}>
                          <td>{user.email}</td>
                          <td>
                            {showPasswordForm === user.id ? (
                              <div className="password-form">
                                <input
                                  type="text"
                                  value={newPassword}
                                  onChange={(e) => setNewPassword(e.target.value)}
                                  placeholder="Yeni şifre"
                                />
                                <button
                                  type="button"
                                  onClick={() => setNewPassword(generatePassword())}
                                  className="generate-btn"
                                >
                                  Yenile
                                </button>
                                <button onClick={() => handlePasswordChange(user.id)}>
                                  Kaydet
                                </button>
                                <button onClick={() => { setShowPasswordForm(null); setNewPassword(''); }}>
                                  İptal
                                </button>
                              </div>
                            ) : (
                              <>
                                <button onClick={() => setShowPasswordForm(user.id)}>
                                  Şifre
                                </button>
                                <button
                                  onClick={() => handleDelete(user.id)}
                                  className="delete-btn"
                                >
                                  Sil
                                </button>
                              </>
                            )}
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </>
          ) : (
            <div className="empty-state">Domain seçin</div>
          )}
        </div>
      </div>
    </div>
  );
}

export default UsersTab;
