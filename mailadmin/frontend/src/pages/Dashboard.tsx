import { useState } from 'react';
import DomainsTab from '../components/DomainsTab';
import UsersTab from '../components/UsersTab';
import AliasesTab from '../components/AliasesTab';
import SenderPermissionsTab from '../components/SenderPermissionsTab';
import MailingListsTab from '../components/MailingListsTab';
import SSLTab from '../components/SSLTab';

interface DashboardProps {
  onLogout: () => void;
}

type Tab = 'domains' | 'users' | 'aliases' | 'permissions' | 'lists' | 'ssl';

function Dashboard({ onLogout }: DashboardProps) {
  const [activeTab, setActiveTab] = useState<Tab>('users');

  const handleLogout = () => {
    localStorage.removeItem('token');
    onLogout();
  };

  return (
    <div className="dashboard">
      <header className="dashboard-header">
        <h1>Mail Admin</h1>
        <button onClick={handleLogout} className="logout-btn">
          Çıkış
        </button>
      </header>

      <nav className="tabs">
        <button
          className={activeTab === 'domains' ? 'active' : ''}
          onClick={() => setActiveTab('domains')}
        >
          Domainler
        </button>
        <button
          className={activeTab === 'users' ? 'active' : ''}
          onClick={() => setActiveTab('users')}
        >
          Kullanıcılar
        </button>
        <button
          className={activeTab === 'aliases' ? 'active' : ''}
          onClick={() => setActiveTab('aliases')}
        >
          Aliaslar
        </button>
        <button
          className={activeTab === 'permissions' ? 'active' : ''}
          onClick={() => setActiveTab('permissions')}
        >
          Gönderim İzinleri
        </button>
        <button
          className={activeTab === 'lists' ? 'active' : ''}
          onClick={() => setActiveTab('lists')}
        >
          Mailing Listler
        </button>
        <button
          className={activeTab === 'ssl' ? 'active' : ''}
          onClick={() => setActiveTab('ssl')}
        >
          SSL Sertifikalar
        </button>
      </nav>

      <main className="tab-content">
        {activeTab === 'domains' && <DomainsTab />}
        {activeTab === 'users' && <UsersTab />}
        {activeTab === 'aliases' && <AliasesTab />}
        {activeTab === 'permissions' && <SenderPermissionsTab />}
        {activeTab === 'lists' && <MailingListsTab />}
        {activeTab === 'ssl' && <SSLTab />}
      </main>
    </div>
  );
}

export default Dashboard;
