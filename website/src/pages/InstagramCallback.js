// InstagramCallback.js
import React, { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import './Login.css';

function InstagramCallback() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [status, setStatus] = useState('processing'); // 'processing', 'success', 'error'
  const [message, setMessage] = useState('Procesando autorización de Instagram Business...');

  useEffect(() => {
    const code = searchParams.get('code');
    const state = searchParams.get('state');
    const error = searchParams.get('error');
    const errorDescription = searchParams.get('error_description');

    console.log('Instagram callback received:', { code, state, error, errorDescription });

    if (error) {
      console.error('Instagram authorization error:', error, errorDescription);
      setStatus('error');
      setMessage(`Error de autorización: ${errorDescription || error}`);
      return;
    }

    if (!code || state !== 'instagram_business_auth') {
      console.error('Invalid Instagram callback parameters');
      setStatus('error');
      setMessage('Parámetros de autorización inválidos');
      return;
    }

    // Exchange authorization code for access token
    exchangeCodeForToken(code);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams]);

  const exchangeCodeForToken = async (code) => {
    try {
      setMessage('Intercambiando código de autorización por token de acceso...');

      // Send the authorization code to our backend for token exchange
      // This keeps the app secret secure on the server side
      const response = await fetch('https://neurocrow-client-manager.onrender.com/instagram-token-exchange', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          code: code,
          redirect_uri: window.location.origin + '/instagram-callback'
        }),
      });

      const data = await response.json();
      console.log('Instagram token exchange response:', data);

      if (!response.ok) {
        throw new Error(data.error || `HTTP ${response.status}: ${response.statusText}`);
      }

      if (!data.access_token) {
        throw new Error('No access token received from server');
      }

      setMessage('Token recibido exitosamente. Redirigiendo...');
      
      // Navigate to success page with the token
      navigate('/success', { 
        state: { 
          accessToken: data.access_token,
          authType: 'instagram'
        } 
      });

    } catch (error) {
      console.error('Error exchanging code for token:', error);
      setStatus('error');
      setMessage(`Error obteniendo token de acceso: ${error.message}`);
    }
  };

  const handleRetry = () => {
    navigate('/login');
  };

  return (
    <div className="login-container">
      <div className="login-box">
        <h1>Autorización Instagram Business</h1>
        
        {status === 'processing' && (
          <div className="verification-message">
            <div className="loading-spinner">
              <i className="fas fa-spinner fa-spin"></i>
            </div>
            <p>{message}</p>
          </div>
        )}

        {status === 'error' && (
          <div className="error-message">
            <i className="fas fa-exclamation-triangle"></i>
            <h3>Error de Autorización</h3>
            <p>{message}</p>
            <button onClick={handleRetry} className="retry-btn">
              Intentar de nuevo
            </button>
          </div>
        )}

        {status === 'success' && (
          <div className="success-message">
            <i className="fas fa-check-circle"></i>
            <h3>¡Autorización Exitosa!</h3>
            <p>Redirigiendo a la aplicación...</p>
          </div>
        )}
      </div>
    </div>
  );
}

export default InstagramCallback;