// Success.js
import React, { useEffect, useState } from 'react';
import { useLocation } from 'react-router-dom';
import './Success.css';

function Success() {
  const [syncStatus, setSyncStatus] = useState('syncing');
  const [setupProgress, setSetupProgress] = useState({
    pageConnection: 'pending',  // pending, success, error
    webhookSetup: 'pending',    // pending, success, error  
    handoverConfig: 'pending'   // pending, success, error
  });
  const location = useLocation();
  const accessToken = location.state?.accessToken;

  useEffect(() => {
    if (!accessToken) {
      console.error('No access token available');
      setSyncStatus('error');
      setSetupProgress({
        pageConnection: 'error',
        webhookSetup: 'error', 
        handoverConfig: 'error'
      });
      return;
    }

    // Update progress indicators step by step
    setSetupProgress(prev => ({ ...prev, pageConnection: 'in_progress' }));

    // Send token to your backend
    fetch('https://neurocrow-client-manager.onrender.com/facebook-token', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ userToken: accessToken }),
    })
    .then(response => {
      if (!response.ok) {
        throw new Error('Network response was not ok');
      }
      return response.json();
    })
    .then(data => {
      if (data.success && data.session_token) {
        // Store session token for authenticated requests
        localStorage.setItem('session_token', data.session_token);
        localStorage.setItem('client_id', data.client_id);
        localStorage.setItem('facebook_connected', 'true');
        
        // Simulate progress through the setup steps
        setSetupProgress(prev => ({ ...prev, pageConnection: 'success' }));
        
        // Simulate webhook setup (in reality this happens in backend)
        setTimeout(() => {
          setSetupProgress(prev => ({ ...prev, webhookSetup: 'in_progress' }));
          
          setTimeout(() => {
            setSetupProgress(prev => ({ ...prev, webhookSetup: 'success' }));
            
            // Simulate handover protocol setup
            setTimeout(() => {
              setSetupProgress(prev => ({ ...prev, handoverConfig: 'in_progress' }));
              
              setTimeout(() => {
                setSetupProgress(prev => ({ ...prev, handoverConfig: 'success' }));
                setSyncStatus('success');
              }, 1000);
            }, 1000);
          }, 1500);
        }, 1000);
      } else {
        throw new Error('Authentication failed - no session token received');
      }
    })
    .catch(error => {
      console.error('Error syncing pages:', error);
      setSyncStatus('error');
      setSetupProgress(prev => ({
        pageConnection: prev.pageConnection === 'in_progress' ? 'error' : prev.pageConnection,
        webhookSetup: 'error',
        handoverConfig: 'error'
      }));
    });
  }, [accessToken]);

  const handleContactClick = () => {
    window.open('https://m.me/413548765185533', '_blank');
  };

  const getStatusIcon = (status) => {
    switch (status) {
      case 'pending':
        return <i className="fas fa-clock" style={{ color: '#ccc' }}></i>;
      case 'in_progress':
        return <i className="fas fa-spinner fa-spin" style={{ color: '#007bff' }}></i>;
      case 'success':
        return <i className="fas fa-check-circle" style={{ color: '#28a745' }}></i>;
      case 'error':
        return <i className="fas fa-times-circle" style={{ color: '#dc3545' }}></i>;
      default:
        return <i className="fas fa-clock" style={{ color: '#ccc' }}></i>;
    }
  };

  return (
    <div className="success-container">
      <div className="success-box">
        {syncStatus === 'syncing' ? (
          <>
            <i className="fas fa-spinner fa-spin success-icon"></i>
            <h1>Configurando tu cuenta...</h1>
            <p>Estamos configurando automáticamente tus cuentas para que funcionen con Neurocrow.</p>
            
            <div className="setup-progress" style={{ margin: '20px 0', textAlign: 'left' }}>
              <div className="progress-item" style={{ display: 'flex', alignItems: 'center', margin: '10px 0' }}>
                {getStatusIcon(setupProgress.pageConnection)}
                <span style={{ marginLeft: '10px' }}>Conectando páginas de Facebook/Instagram</span>
              </div>
              <div className="progress-item" style={{ display: 'flex', alignItems: 'center', margin: '10px 0' }}>
                {getStatusIcon(setupProgress.webhookSetup)}
                <span style={{ marginLeft: '10px' }}>Configurando webhooks (Facebook API + Instagram app-level)</span>
              </div>
              <div className="progress-item" style={{ display: 'flex', alignItems: 'center', margin: '10px 0' }}>
                {getStatusIcon(setupProgress.handoverConfig)}
                <span style={{ marginLeft: '10px' }}>Configurando protocolo avanzado (solo Facebook)</span>
              </div>
            </div>
          </>
        ) : syncStatus === 'success' ? (
          <>
            <i className="fas fa-check-circle success-icon"></i>
            <h1>¡Configuración Completada!</h1>
            <p>Tu cuenta ha sido configurada automáticamente. Tus páginas de Facebook e Instagram ya están listas para recibir mensajes y usar el chatbot de Neurocrow.</p>
            
            <div className="setup-summary" style={{ margin: '20px 0', padding: '15px', backgroundColor: '#f8f9fa', borderRadius: '8px', textAlign: 'left' }}>
              <h3 style={{ margin: '0 0 10px 0', color: '#28a745' }}>✅ Configuración completada:</h3>
              <ul style={{ margin: '0', paddingLeft: '20px' }}>
                <li>📱 Páginas de Facebook e Instagram conectadas</li>
                <li>🔗 Webhooks Facebook configurados vía API</li>
                <li>📲 Webhooks Instagram configurados a nivel de app</li>
                <li>📨 Protocolo de mensajería activado (Facebook)</li>
                <li>🤖 Bot listo para responder mensajes en ambas plataformas</li>
                <li>📱 Posts y contenido de página disponibles</li>
                <li>📝 <em>Instagram funciona automáticamente vía configuración central</em></li>
              </ul>
            </div>
            
            <div className="next-steps" style={{ margin: '20px 0' }}>
              <a 
                href="/insights" 
                className="insights-btn"
                style={{
                  display: 'inline-block',
                  background: '#667eea',
                  color: 'white',
                  padding: '12px 24px',
                  borderRadius: '8px',
                  textDecoration: 'none',
                  fontWeight: '600',
                  marginRight: '15px',
                  transition: 'all 0.3s ease'
                }}
                onMouseOver={(e) => e.target.style.background = '#5a6fd8'}
                onMouseOut={(e) => e.target.style.background = '#667eea'}
              >
                <i className="fas fa-rss"></i> Ver Últimos Posts
              </a>
            </div>
          </>
        ) : (
          <>
            <i className="fas fa-exclamation-circle success-icon error"></i>
            <h1>Hubo un problema</h1>
            <p>No pudimos completar la configuración automática. Por favor contáctanos para ayudarte.</p>
          </>
        )}
        
        <div className="contact-options">
          <p>Si tienes alguna pregunta, puedes contactarnos por:</p>
          <div className="contact-buttons">
            <button onClick={handleContactClick} className="messenger-btn">
              <i className="fab fa-facebook-messenger"></i> Messenger
            </button>
            <a 
              href="https://wa.me/+16197612314" 
              target="_blank" 
              rel="noopener noreferrer" 
              className="whatsapp-btn"
            >
              <i className="fab fa-whatsapp"></i> WhatsApp
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}

export default Success;