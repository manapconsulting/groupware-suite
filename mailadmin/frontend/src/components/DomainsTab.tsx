import { useState, useEffect } from 'react';
import { domains, dnsCheck, clientSetup } from '../api';
import type { Domain, DomainCheckResponse, ClientSetupInfo } from '../api';
import DomainApiKeys from './DomainApiKeys';

function DomainsTab() {
  const [domainList, setDomainList] = useState<Domain[]>([]);
  const [selectedDomain, setSelectedDomain] = useState<Domain | null>(null);
  const [newDomain, setNewDomain] = useState('');
  const [loading, setLoading] = useState(true);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState('');
  const [checkResults, setCheckResults] = useState<DomainCheckResponse | null>(null);
  const [setupInfo, setSetupInfo] = useState<ClientSetupInfo | null>(null);
  const [showAddForm, setShowAddForm] = useState(false);
  const [activeTab, setActiveTab] = useState<'dns' | 'setup' | 'apikeys'>('dns');
  const [expandedClient, setExpandedClient] = useState<string | null>(null);

  const fetchDomains = async () => {
    try {
      const response = await domains.list();
      if (response.data.success) {
        setDomainList(response.data.data || []);
      }
    } catch (err) {
      setError('Domain listesi alinamadi');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDomains();
  }, []);

  const runDNSCheck = async (domain: string) => {
    setChecking(true);
    setCheckResults(null);
    try {
      const response = await dnsCheck.check(domain);
      if (response.data.success && response.data.data) {
        setCheckResults(response.data.data);
      }
    } catch (err) {
      setError('DNS kontrolu yapilamadi');
    } finally {
      setChecking(false);
    }
  };

  const fetchClientSetup = async (domain: string) => {
    try {
      const response = await clientSetup.get(domain);
      if (response.data.success && response.data.data) {
        setSetupInfo(response.data.data);
      }
    } catch (err) {
      setError('Kurulum bilgisi alinamadi');
    }
  };

  const handleSelectDomain = (domain: Domain) => {
    setSelectedDomain(domain);
    setActiveTab('dns');
    runDNSCheck(domain.name);
    fetchClientSetup(domain.name);
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newDomain.trim()) return;

    try {
      const response = await domains.create(newDomain);
      if (response.data.success && response.data.data) {
        setShowAddForm(false);
        await fetchDomains();
        setSelectedDomain(response.data.data);
        runDNSCheck(newDomain);
        fetchClientSetup(newDomain);
        setNewDomain('');
      } else {
        setError(response.data.message || 'Olusturulamadi');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Olusturulamadi');
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Bu domaini silmek istediginize emin misiniz?')) return;

    try {
      const response = await domains.delete(id);
      if (response.data.success) {
        if (selectedDomain?.id === id) {
          setSelectedDomain(null);
          setCheckResults(null);
          setSetupInfo(null);
        }
        fetchDomains();
      } else {
        setError(response.data.message || 'Silinemedi');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Silinemedi');
    }
  };

  const handleServiceToggle = async (domain: Domain, service: 'webmail' | 'caldav' | 'carddav') => {
    try {
      const updateData: Partial<Domain> = {
        webmail_enabled: domain.webmail_enabled,
        caldav_enabled: domain.caldav_enabled,
        carddav_enabled: domain.carddav_enabled,
      };

      // Toggle the specific service
      if (service === 'webmail') updateData.webmail_enabled = !domain.webmail_enabled;
      if (service === 'caldav') updateData.caldav_enabled = !domain.caldav_enabled;
      if (service === 'carddav') updateData.carddav_enabled = !domain.carddav_enabled;

      const response = await domains.update(domain.id, updateData);
      if (response.data.success) {
        const updatedDomain = { ...domain, ...updateData };
        setDomainList((prev) =>
          prev.map((d) => d.id === domain.id ? updatedDomain : d)
        );
        if (selectedDomain?.id === domain.id) {
          setSelectedDomain(updatedDomain);
        }
      } else {
        setError(response.data.message || 'Guncellenemedi');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Guncellenemedi');
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'ok':
        return '✓';
      case 'warning':
        return '⚠';
      case 'error':
        return '✗';
      default:
        return '?';
    }
  };

  const getStatusClass = (status: string) => {
    switch (status) {
      case 'ok':
        return 'status-ok';
      case 'warning':
        return 'status-warning';
      case 'error':
        return 'status-error';
      default:
        return '';
    }
  };

  const renderWithLinks = (text: string) => {
    const urlRegex = /(https?:\/\/[^\s]+)/g;
    const parts = text.split(urlRegex);

    return parts.map((part, index) => {
      if (part.match(urlRegex)) {
        return (
          <a
            key={index}
            href={part}
            target="_blank"
            rel="noopener noreferrer"
            className="help-link"
          >
            {part}
          </a>
        );
      }
      return part.split('\n').map((line, lineIndex) => (
        <span key={`${index}-${lineIndex}`}>
          {lineIndex > 0 && <br />}
          {line}
        </span>
      ));
    });
  };

  const getClientIcon = (icon: string) => {
    switch (icon) {
      case 'outlook':
        return '📧';
      case 'thunderbird':
        return '🦊';
      case 'apple':
        return '🍎';
      case 'ios':
        return '📱';
      case 'android':
        return '🤖';
      case 'windows':
        return '🪟';
      default:
        return '📨';
    }
  };

  const handlePrint = () => {
    window.print();
  };

  if (loading) return <div className="loading">Yukleniyor...</div>;

  return (
    <div className="tab-panel">
      <h2>Domainler</h2>

      {error && <div className="error-message">{error}</div>}

      <div className="domains-container">
        <div className="domains-sidebar">
          <div className="sidebar-header">
            <h3>Domainler</h3>
            <button
              onClick={() => setShowAddForm(!showAddForm)}
              className="add-btn small"
            >
              {showAddForm ? 'Iptal' : '+ Ekle'}
            </button>
          </div>

          {showAddForm && (
            <form onSubmit={handleCreate} className="domain-add-form">
              <input
                type="text"
                value={newDomain}
                onChange={(e) => setNewDomain(e.target.value)}
                placeholder="yenidomain.com"
                autoFocus
              />
              <button type="submit">Ekle</button>
            </form>
          )}

          <ul className="domain-items">
            {domainList.map((domain) => (
              <li
                key={domain.id}
                className={selectedDomain?.id === domain.id ? 'active' : ''}
                onClick={() => handleSelectDomain(domain)}
              >
                <span className="domain-name">{domain.name}</span>
              </li>
            ))}
          </ul>
        </div>

        <div className="domain-detail">
          {selectedDomain ? (
            <>
              <div className="domain-header">
                <h3>{selectedDomain.name}</h3>
                <div className="domain-actions">
                  {activeTab === 'dns' && (
                    <button
                      onClick={() => runDNSCheck(selectedDomain.name)}
                      className="refresh-btn"
                      disabled={checking}
                    >
                      {checking ? 'Kontrol ediliyor...' : 'Yeniden Kontrol'}
                    </button>
                  )}
                  {activeTab === 'setup' && (
                    <button onClick={handlePrint} className="refresh-btn">
                      Yazdir / PDF
                    </button>
                  )}
                  <button
                    onClick={() => handleDelete(selectedDomain.id)}
                    className="delete-btn"
                  >
                    Sil
                  </button>
                </div>
              </div>

              <div className="domain-settings">
                <div className="service-toggles">
                  <label className="toggle-switch">
                    <input
                      type="checkbox"
                      checked={selectedDomain.webmail_enabled}
                      onChange={() => handleServiceToggle(selectedDomain, 'webmail')}
                    />
                    <span className="toggle-slider"></span>
                    <span className="toggle-label">Webmail</span>
                  </label>
                  <label className="toggle-switch">
                    <input
                      type="checkbox"
                      checked={selectedDomain.caldav_enabled}
                      onChange={() => handleServiceToggle(selectedDomain, 'caldav')}
                    />
                    <span className="toggle-slider"></span>
                    <span className="toggle-label">Takvim (CalDAV)</span>
                  </label>
                  <label className="toggle-switch">
                    <input
                      type="checkbox"
                      checked={selectedDomain.carddav_enabled}
                      onChange={() => handleServiceToggle(selectedDomain, 'carddav')}
                    />
                    <span className="toggle-slider"></span>
                    <span className="toggle-label">Rehber (CardDAV)</span>
                  </label>
                </div>
                {selectedDomain.webmail_enabled && (
                  <a
                    href="/mail/"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="webmail-link"
                  >
                    Webmail'e Git
                  </a>
                )}
              </div>

              <div className="detail-tabs">
                <button
                  className={activeTab === 'dns' ? 'active' : ''}
                  onClick={() => setActiveTab('dns')}
                >
                  DNS Kontrolu
                </button>
                <button
                  className={activeTab === 'setup' ? 'active' : ''}
                  onClick={() => setActiveTab('setup')}
                >
                  Istemci Kurulumu
                </button>
                <button
                  className={activeTab === 'apikeys' ? 'active' : ''}
                  onClick={() => setActiveTab('apikeys')}
                >
                  API Anahtarlari
                </button>
              </div>

              {activeTab === 'dns' && (
                <>
                  {checking && (
                    <div className="checking-status">
                      DNS ayarlari kontrol ediliyor...
                    </div>
                  )}

                  {checkResults && (
                    <div className="dns-check-results">
                      <div className="server-info">
                        <span>Sunucu: <strong>{checkResults.hostname}</strong></span>
                        <span>IP: <strong>{checkResults.server_ip}</strong></span>
                      </div>

                      <div className={`overall-status ${getStatusClass(checkResults.overall)}`}>
                        <span className="status-icon">{getStatusIcon(checkResults.overall)}</span>
                        <span className="status-text">
                          {checkResults.overall === 'ok' && 'Tum ayarlar dogru!'}
                          {checkResults.overall === 'warning' && 'Bazi ayarlar eksik'}
                          {checkResults.overall === 'error' && 'Kritik ayarlar eksik!'}
                        </span>
                      </div>

                      <div className="check-items">
                        {checkResults.checks.map((check, index) => (
                          <div key={index} className={`check-item ${getStatusClass(check.status)}`}>
                            <div className="check-header">
                              <span className="check-icon">{getStatusIcon(check.status)}</span>
                              <span className="check-name">{check.check}</span>
                            </div>
                            <div className="check-message">{check.message}</div>
                            {check.value && (
                              <div className="check-value">
                                <code>{check.value}</code>
                              </div>
                            )}
                            {check.help && (
                              <div className="check-help">
                                <strong>Oneri:</strong> {renderWithLinks(check.help)}
                              </div>
                            )}
                            {check.suggested && (
                              <div className="check-suggested">
                                <div className="suggested-label">DNS Kaydi (kopyalayin):</div>
                                <code>{check.suggested}</code>
                                <button
                                  className="copy-btn"
                                  onClick={() => {
                                    navigator.clipboard.writeText(check.suggested || '');
                                    alert('Kopyalandi!');
                                  }}
                                >
                                  Kopyala
                                </button>
                              </div>
                            )}
                          </div>
                        ))}
                      </div>

                      <div className="dns-info-box">
                        <h4>DNS Kayitlari Nasil Eklenir?</h4>
                        <p>
                          DNS kayitlarini domain saglayicinizin (Godaddy, Namecheap, Cloudflare vb.)
                          kontrol panelinden ekleyebilirsiniz. Degisikliklerin yayilmasi 24 saate
                          kadar surebilir.
                        </p>
                      </div>
                    </div>
                  )}
                </>
              )}

              {activeTab === 'setup' && setupInfo && (
                <div className="client-setup-content printable">
                  <div className="setup-header-print">
                    <h2>E-posta Istemci Kurulum Kilavuzu</h2>
                    <p className="domain-print">{selectedDomain.name}</p>
                  </div>

                  <div className="server-settings">
                    <h4>Sunucu Ayarlari</h4>
                    <div className="settings-table">
                      <table>
                        <thead>
                          <tr>
                            <th>Protokol</th>
                            <th>Sunucu</th>
                            <th>Port</th>
                            <th>Guvenlik</th>
                          </tr>
                        </thead>
                        <tbody>
                          <tr>
                            <td><strong>IMAP</strong> (Onerilen)</td>
                            <td>{setupInfo.imap.server}</td>
                            <td>{setupInfo.imap.port}</td>
                            <td>{setupInfo.imap.security}</td>
                          </tr>
                          <tr>
                            <td><strong>POP3</strong></td>
                            <td>{setupInfo.pop3.server}</td>
                            <td>{setupInfo.pop3.port}</td>
                            <td>{setupInfo.pop3.security}</td>
                          </tr>
                          <tr>
                            <td><strong>SMTP</strong> (Giden)</td>
                            <td>{setupInfo.smtp.server}</td>
                            <td>{setupInfo.smtp.port}{setupInfo.smtp.port_alt ? ` / ${setupInfo.smtp.port_alt}` : ''}</td>
                            <td>{setupInfo.smtp.security}</td>
                          </tr>
                        </tbody>
                      </table>
                    </div>

                    <div className="credentials-note">
                      <strong>Kullanici Bilgileri:</strong>
                      <ul>
                        <li>Kullanici adi: Tam e-posta adresiniz (ornegin: isim@{selectedDomain.name})</li>
                        <li>Sifre: E-posta hesap sifreniz</li>
                        <li>Kimlik dogrulama: Normal sifre</li>
                      </ul>
                    </div>
                  </div>

                  <div className="client-guides">
                    <h4>Istemci Kurulum Talimatlari</h4>

                    {setupInfo.clients.map((client) => (
                      <div key={client.name} className="client-guide">
                        <div
                          className="client-guide-header"
                          onClick={() => setExpandedClient(expandedClient === client.name ? null : client.name)}
                        >
                          <span className="client-icon">{getClientIcon(client.icon)}</span>
                          <div className="client-info">
                            <span className="client-name">{client.name}</span>
                            <span className="client-platform">{client.platform}</span>
                          </div>
                          <span className="expand-icon">{expandedClient === client.name ? '▼' : '▶'}</span>
                        </div>

                        {(expandedClient === client.name || true) && (
                          <div className={`client-steps ${expandedClient === client.name ? 'expanded' : 'collapsed'}`}>
                            <ol>
                              {client.steps.map((step, index) => (
                                <li key={index}>{step}</li>
                              ))}
                            </ol>
                          </div>
                        )}
                      </div>
                    ))}
                  </div>

                  <div className="setup-footer">
                    <p>
                      <strong>Onemli:</strong> IMAP protokolu, e-postalarinizi sunucuda tutarak birden fazla cihazda senkronize erisim saglar.
                      POP3 ise e-postalari indirir ve genellikle sunucudan siler.
                    </p>
                  </div>
                </div>
              )}

              {activeTab === 'apikeys' && (
                <DomainApiKeys domain={selectedDomain} />
              )}
            </>
          ) : (
            <div className="empty-state">
              <p>DNS ayarlarini kontrol etmek veya kurulum bilgilerini gormek icin bir domain secin</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default DomainsTab;
