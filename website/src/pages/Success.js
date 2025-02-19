// Success.js
import React, { useEffect, useState } from 'react';
import { useLocation } from 'react-router-dom';
import './Success.css';

function Success() {
  const [syncStatus, setSyncStatus] = useState('syncing');
  const location = useLocation();
  const accessToken = location.state?.accessToken;

  useEffect(() => {
    if (!accessToken) {
      console.error('No access token available');
      setSyncStatus('error');
      return;
    }

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
      setSyncStatus('success');
    })
    .catch(error => {
      console.error('Error syncing pages:', error);
      setSyncStatus('error');
    });
  }, [accessToken]);

  const handleContactClick = () => {
    window.open('https://m.me/413548765185533', '_blank');
  };

  return (
    <div className="success-container">
      <div className="success-box">
        {syncStatus === 'syncing' ? (
          <>
            <i className="fas fa-spinner fa-spin success-icon"></i>
            <h1>Sincronizando páginas...</h1>
            <p>Estamos configurando tus cuentas conectadas. Por favor espera un momento.</p>
          </>
        ) : syncStatus === 'success' ? (
          <>
            <i className="fas fa-check-circle success-icon"></i>
            <h1>¡Conexión Exitosa!</h1>
            <p>Gracias por conectar tus cuentas con Neurocrow. Nos pondremos en contacto contigo pronto para configurar tu chatbot.</p>
          </>
        ) : (
          <>
            <i className="fas fa-exclamation-circle success-icon error"></i>
            <h1>Hubo un problema</h1>
            <p>No pudimos sincronizar tus páginas. Por favor contáctanos para ayudarte.</p>
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