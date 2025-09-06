// InstagramCallback.js
import React, { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';

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
      const response = await fetch('https://neurocrow-message-router.onrender.com/instagram-token-exchange', {
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
    <div className="min-h-screen bg-slate-50 dark:bg-slate-900 flex items-center justify-center p-4">
      <div className="w-full max-w-md bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 p-8 space-y-6">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100">Autorización Instagram Business</h1>
        </div>
        
        {status === 'processing' && (
          <div className="text-center space-y-4">
            <div className="w-12 h-12 mx-auto mb-4 text-slate-600">
              <i className="fas fa-spinner fa-spin text-2xl"></i>
            </div>
            <p className="text-slate-600 dark:text-slate-300">{message}</p>
          </div>
        )}

        {status === 'error' && (
          <div className="text-center space-y-4">
            <i className="fas fa-exclamation-triangle text-3xl text-red-500 mb-4"></i>
            <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Error de Autorización</h3>
            <p className="text-slate-600 dark:text-slate-300">{message}</p>
            <button onClick={handleRetry} className="w-full px-4 py-2 bg-red-600 hover:bg-red-700 text-white font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2">
              Intentar de nuevo
            </button>
          </div>
        )}

        {status === 'success' && (
          <div className="text-center space-y-4">
            <i className="fas fa-check-circle text-3xl text-green-500 mb-4"></i>
            <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">¡Autorización Exitosa!</h3>
            <p className="text-slate-600">Redirigiendo a la aplicación...</p>
          </div>
        )}
      </div>
    </div>
  );
}

export default InstagramCallback;