import { useState, useEffect } from 'react';
import { mailingLists, domains } from '../api';
import type { MailingList, ListMember, ListSettings, Domain } from '../api';

function MailingListsTab() {
  const [lists, setLists] = useState<MailingList[]>([]);
  const [domainList, setDomainList] = useState<Domain[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [selectedList, setSelectedList] = useState<string | null>(null);
  const [members, setMembers] = useState<ListMember[]>([]);
  const [membersLoading, setMembersLoading] = useState(false);
  const [showMemberForm, setShowMemberForm] = useState(false);
  const [newMemberEmail, setNewMemberEmail] = useState('');
  const [activeDetailTab, setActiveDetailTab] = useState<'members' | 'settings'>('members');
  const [settings, setSettings] = useState<ListSettings | null>(null);
  const [settingsLoading, setSettingsLoading] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    domain: '',
    description: '',
  });

  const fetchLists = async () => {
    try {
      const [listsRes, domainsRes] = await Promise.all([
        mailingLists.list(),
        domains.list(),
      ]);
      if (listsRes.data.success) {
        setLists(listsRes.data.data || []);
      }
      if (domainsRes.data.success) {
        setDomainList(domainsRes.data.data || []);
        if (domainsRes.data.data && domainsRes.data.data.length > 0) {
          setFormData((prev) => ({ ...prev, domain: domainsRes.data.data![0].name }));
        }
      }
    } catch (err) {
      setError('Veriler alınamadı');
    } finally {
      setLoading(false);
    }
  };

  const fetchMembers = async (listId: string) => {
    setMembersLoading(true);
    try {
      const res = await mailingLists.getMembers(listId);
      if (res.data.success) {
        setMembers(res.data.data || []);
      }
    } catch (err) {
      setError('Üyeler alınamadı');
    } finally {
      setMembersLoading(false);
    }
  };

  const fetchSettings = async (listId: string) => {
    setSettingsLoading(true);
    try {
      const res = await mailingLists.getSettings(listId);
      if (res.data.success && res.data.data) {
        setSettings(res.data.data);
      }
    } catch (err) {
      setError('Ayarlar alınamadı');
    } finally {
      setSettingsLoading(false);
    }
  };

  useEffect(() => {
    fetchLists();
  }, []);

  useEffect(() => {
    if (selectedList) {
      fetchMembers(selectedList);
      fetchSettings(selectedList);
    }
  }, [selectedList]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    try {
      const response = await mailingLists.create(formData);
      if (response.data.success) {
        setShowForm(false);
        setFormData({ name: '', domain: domainList[0]?.name || '', description: '' });
        fetchLists();
      } else {
        setError(response.data.message || 'Oluşturulamadı');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Oluşturulamadı');
    }
  };

  const handleDelete = async (listId: string) => {
    if (!confirm('Bu listeyi silmek istediğinize emin misiniz?')) return;

    try {
      const response = await mailingLists.delete(listId);
      if (response.data.success) {
        if (selectedList === listId) {
          setSelectedList(null);
          setMembers([]);
          setSettings(null);
        }
        fetchLists();
      } else {
        setError(response.data.message || 'Silinemedi');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Silinemedi');
    }
  };

  const handleAddMember = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedList || !newMemberEmail.trim()) return;

    try {
      const response = await mailingLists.addMember(selectedList, { email: newMemberEmail });
      if (response.data.success) {
        setShowMemberForm(false);
        setNewMemberEmail('');
        fetchMembers(selectedList);
        fetchLists();
      } else {
        setError(response.data.message || 'Üye eklenemedi');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Üye eklenemedi');
    }
  };

  const handleRemoveMember = async (email: string) => {
    if (!selectedList) return;
    if (!confirm(`${email} adresini listeden çıkarmak istediğinize emin misiniz?`)) return;

    try {
      const response = await mailingLists.removeMember(selectedList, email);
      if (response.data.success) {
        fetchMembers(selectedList);
        fetchLists();
      } else {
        setError(response.data.message || 'Üye çıkarılamadı');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Üye çıkarılamadı');
    }
  };

  const handleSaveSettings = async () => {
    if (!selectedList || !settings) return;

    try {
      const response = await mailingLists.updateSettings(selectedList, settings);
      if (response.data.success) {
        alert('Ayarlar kaydedildi');
      } else {
        setError(response.data.message || 'Ayarlar kaydedilemedi');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Ayarlar kaydedilemedi');
    }
  };

  if (loading) return <div className="loading">Yükleniyor...</div>;

  const selectedListData = lists.find((l) => l.list_id === selectedList);

  return (
    <div className="tab-panel">
      <h2>Mailing Listler</h2>

      {error && <div className="error-message">{error}</div>}

      <button onClick={() => setShowForm(!showForm)} className="add-btn">
        {showForm ? 'İptal' : 'Yeni Liste'}
      </button>

      {showForm && (
        <form onSubmit={handleCreate} className="add-form vertical">
          <div className="form-group">
            <label>Liste Adı</label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              placeholder="duyuru"
              required
            />
          </div>
          <div className="form-group">
            <label>Domain</label>
            <select
              value={formData.domain}
              onChange={(e) => setFormData({ ...formData, domain: e.target.value })}
            >
              {domainList.map((d) => (
                <option key={d.id} value={d.name}>
                  {d.name}
                </option>
              ))}
            </select>
          </div>
          <div className="form-group">
            <label>Açıklama</label>
            <input
              type="text"
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              placeholder="Liste açıklaması"
            />
          </div>
          <button type="submit">Oluştur</button>
        </form>
      )}

      <div className="lists-container">
        <div className="lists-sidebar">
          <h3>Listeler</h3>
          {lists.length === 0 ? (
            <p className="info-text">Henüz liste yok</p>
          ) : (
            <ul className="list-items">
              {lists.map((list) => (
                <li
                  key={list.list_id}
                  className={selectedList === list.list_id ? 'active' : ''}
                  onClick={() => setSelectedList(list.list_id)}
                >
                  <div className="list-item-content">
                    <strong>{list.email}</strong>
                    <span className="member-count">{list.member_count} üye</span>
                  </div>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      handleDelete(list.list_id);
                    }}
                    className="delete-btn small"
                  >
                    Sil
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>

        {selectedList && selectedListData && (
          <div className="list-detail">
            <h3>{selectedListData.email}</h3>

            <div className="detail-tabs">
              <button
                className={activeDetailTab === 'members' ? 'active' : ''}
                onClick={() => setActiveDetailTab('members')}
              >
                Üyeler
              </button>
              <button
                className={activeDetailTab === 'settings' ? 'active' : ''}
                onClick={() => setActiveDetailTab('settings')}
              >
                Ayarlar
              </button>
            </div>

            {activeDetailTab === 'members' && (
              <>
                <div className="members-header">
                  <h4>Üyeler ({members.length})</h4>
                  <button onClick={() => setShowMemberForm(!showMemberForm)} className="add-btn small">
                    {showMemberForm ? 'İptal' : 'Üye Ekle'}
                  </button>
                </div>

                {showMemberForm && (
                  <form onSubmit={handleAddMember} className="member-form">
                    <input
                      type="email"
                      value={newMemberEmail}
                      onChange={(e) => setNewMemberEmail(e.target.value)}
                      placeholder="email@domain.com"
                      required
                    />
                    <button type="submit">Ekle</button>
                  </form>
                )}

                {membersLoading ? (
                  <div className="loading">Yükleniyor...</div>
                ) : members.length === 0 ? (
                  <p className="info-text">Henüz üye yok</p>
                ) : (
                  <table>
                    <thead>
                      <tr>
                        <th>E-posta</th>
                        <th>İşlem</th>
                      </tr>
                    </thead>
                    <tbody>
                      {members.map((member) => (
                        <tr key={member.email}>
                          <td>{member.email}</td>
                          <td>
                            <button
                              onClick={() => handleRemoveMember(member.email)}
                              className="delete-btn"
                            >
                              Çıkar
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </>
            )}

            {activeDetailTab === 'settings' && (
              <div className="settings-panel">
                {settingsLoading ? (
                  <div className="loading">Yükleniyor...</div>
                ) : settings ? (
                  <>
                    <div className="settings-section">
                      <h4>Genel Ayarlar</h4>
                      <div className="settings-grid">
                        <div className="form-group">
                          <label>Açıklama</label>
                          <input
                            type="text"
                            value={settings.description}
                            onChange={(e) => setSettings({ ...settings, description: e.target.value })}
                          />
                        </div>
                        <div className="form-group">
                          <label>Konu Öneki</label>
                          <input
                            type="text"
                            value={settings.subject_prefix}
                            onChange={(e) => setSettings({ ...settings, subject_prefix: e.target.value })}
                            placeholder="[Liste]"
                          />
                        </div>
                        <div className="form-group">
                          <label>
                            <input
                              type="checkbox"
                              checked={settings.advertised}
                              onChange={(e) => setSettings({ ...settings, advertised: e.target.checked })}
                            />
                            Listeyi herkese göster
                          </label>
                        </div>
                        <div className="form-group">
                          <label>
                            <input
                              type="checkbox"
                              checked={settings.allow_list_posts}
                              onChange={(e) => setSettings({ ...settings, allow_list_posts: e.target.checked })}
                            />
                            Listeye mesaj göndermeye izin ver
                          </label>
                        </div>
                        <div className="form-group">
                          <label>
                            <input
                              type="checkbox"
                              checked={settings.reply_goes_to_list}
                              onChange={(e) => setSettings({ ...settings, reply_goes_to_list: e.target.checked })}
                            />
                            Yanıtlar listeye gitsin
                          </label>
                        </div>
                      </div>
                    </div>

                    <div className="settings-section">
                      <h4>Digest (Özet) Ayarları</h4>
                      <div className="settings-grid">
                        <div className="form-group">
                          <label>
                            <input
                              type="checkbox"
                              checked={settings.digest_enabled}
                              onChange={(e) => setSettings({ ...settings, digest_enabled: e.target.checked })}
                            />
                            Digest gönderimini etkinleştir
                          </label>
                        </div>
                        <div className="form-group">
                          <label>Digest Sıklığı (gün)</label>
                          <input
                            type="number"
                            min="0.1"
                            step="0.1"
                            value={settings.digest_frequency_days}
                            onChange={(e) => setSettings({ ...settings, digest_frequency_days: parseFloat(e.target.value) || 1 })}
                          />
                        </div>
                      </div>
                    </div>

                    <div className="settings-section">
                      <h4>Üyelik Politikaları</h4>
                      <div className="settings-grid">
                        <div className="form-group">
                          <label>Üyelik Politikası</label>
                          <select
                            value={settings.subscription_policy}
                            onChange={(e) => setSettings({ ...settings, subscription_policy: e.target.value })}
                          >
                            <option value="open">Açık</option>
                            <option value="confirm">Onay Gerekli</option>
                            <option value="moderate">Moderatör Onayı</option>
                            <option value="confirm_then_moderate">Onay + Moderatör</option>
                          </select>
                        </div>
                        <div className="form-group">
                          <label>Üyelikten Ayrılma</label>
                          <select
                            value={settings.unsubscription_policy}
                            onChange={(e) => setSettings({ ...settings, unsubscription_policy: e.target.value })}
                          >
                            <option value="open">Açık</option>
                            <option value="confirm">Onay Gerekli</option>
                            <option value="moderate">Moderatör Onayı</option>
                          </select>
                        </div>
                      </div>
                    </div>

                    <div className="settings-section">
                      <h4>Mesaj Politikaları</h4>
                      <div className="settings-grid">
                        <div className="form-group">
                          <label>Üye Mesajları</label>
                          <select
                            value={settings.default_member_action}
                            onChange={(e) => setSettings({ ...settings, default_member_action: e.target.value })}
                          >
                            <option value="accept">Kabul Et</option>
                            <option value="hold">Beklet (Moderasyon)</option>
                            <option value="reject">Reddet</option>
                            <option value="discard">Sessizce At</option>
                          </select>
                        </div>
                        <div className="form-group">
                          <label>Üye Olmayan Mesajları</label>
                          <select
                            value={settings.default_nonmember_action}
                            onChange={(e) => setSettings({ ...settings, default_nonmember_action: e.target.value })}
                          >
                            <option value="accept">Kabul Et</option>
                            <option value="hold">Beklet (Moderasyon)</option>
                            <option value="reject">Reddet</option>
                            <option value="discard">Sessizce At</option>
                          </select>
                        </div>
                        <div className="form-group">
                          <label>Maks. Mesaj Boyutu (KB)</label>
                          <input
                            type="number"
                            min="0"
                            value={settings.max_message_size}
                            onChange={(e) => setSettings({ ...settings, max_message_size: parseInt(e.target.value) || 0 })}
                          />
                        </div>
                        <div className="form-group">
                          <label>Arşiv Politikası</label>
                          <select
                            value={settings.archive_policy}
                            onChange={(e) => setSettings({ ...settings, archive_policy: e.target.value })}
                          >
                            <option value="public">Herkese Açık</option>
                            <option value="private">Sadece Üyeler</option>
                            <option value="never">Arşivleme</option>
                          </select>
                        </div>
                      </div>
                    </div>

                    <div className="settings-section">
                      <h4>Yönetici Bildirimleri</h4>
                      <div className="settings-grid">
                        <div className="form-group">
                          <label>
                            <input
                              type="checkbox"
                              checked={settings.admin_immed_notify}
                              onChange={(e) => setSettings({ ...settings, admin_immed_notify: e.target.checked })}
                            />
                            Acil durumlarda hemen bildir
                          </label>
                        </div>
                        <div className="form-group">
                          <label>
                            <input
                              type="checkbox"
                              checked={settings.admin_notify_mchanges}
                              onChange={(e) => setSettings({ ...settings, admin_notify_mchanges: e.target.checked })}
                            />
                            Üyelik değişikliklerini bildir
                          </label>
                        </div>
                      </div>
                    </div>

                    <button onClick={handleSaveSettings} className="save-btn">
                      Ayarları Kaydet
                    </button>
                  </>
                ) : (
                  <p className="info-text">Ayarlar yüklenemedi</p>
                )}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

export default MailingListsTab;
