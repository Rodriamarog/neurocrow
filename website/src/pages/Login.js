// Login.js
import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import './Login.css';

function Login() {
  const navigate = useNavigate();
  const [isVerifying, setIsVerifying] = useState(false);
  const [pollInterval, setPollInterval] = useState(null);
  const [connectionType, setConnectionType] = useState('both');

  useEffect(() => {
    return () => {
      if (pollInterval) {
        clearInterval(pollInterval);
      }
    };
  }, [pollInterval]);

  const startPolling = () => {
    let attempts = 0;
    const maxAttempts = 60; // 2 minutes maximum polling time
    
    const interval = setInterval(() => {
      window.FB.getLoginStatus((response) => {
        attempts++;
        
        if (response.status === 'connected') {
          clearInterval(interval);
          setIsVerifying(false);
          const token = response.authResponse.accessToken;
          navigate('/success', { state: { accessToken: token } });
        } else if (attempts >= maxAttempts) {
          clearInterval(interval);
          setIsVerifying(false);
        }
      }, true);
    }, 2000);

    setPollInterval(interval);
  };

  const handleFacebookLogin = () => {
    let scopeArray = [];
    if (connectionType === 'fb' || connectionType === 'both') {
      scopeArray = scopeArray.concat([
        'pages_show_list',
        'pages_manage_metadata',
        'pages_messaging',
        'pages_read_engagement',
        'public_profile'
      ]);
    }
    if (connectionType === 'ig' || connectionType === 'both') {
      scopeArray = scopeArray.concat([
        'pages_show_list',
        'pages_manage_metadata',
        'instagram_basic',
        'instagram_manage_messages',
        'pages_read_engagement',
        'public_profile'
      ]);
    }
    const scope = scopeArray.join(',');

    window.FB.login(function(response) {
      console.log('Login response:', response);
      
      if (response.status === 'connected') {
        const token = response.authResponse.accessToken;
        console.log('Successfully logged in with token');
        navigate('/success', { state: { accessToken: token } });
      } else if (response.status === 'not_authorized') {
        console.log('Awaiting device verification...');
        setIsVerifying(true);
        startPolling();
      } else {
        console.log('User cancelled login or did not fully authorize.');
        setIsVerifying(false);
      }
    }, {
      scope: scope,
      auth_type: 'rerequest'
    });
  };

  return (
    <div className="login-container">
      <div className="login-box">
        <h1>Conecta tus cuentas</h1>
        {isVerifying ? (
          <>
            <div className="verification-message">
              <h2>Verificación pendiente</h2>
              <p>Por favor, aprueba el inicio de sesión en tu aplicación de Facebook.</p>
              <div className="loading-spinner">
                <i className="fas fa-spinner fa-spin"></i>
              </div>
            </div>
          </>
        ) : (
          <>
            <p>Para comenzar, conecta tus cuentas de redes sociales a nuestra app</p>
            <div className="connection-type-selector">
              <label>
                <input 
                  type="radio" 
                  value="fb" 
                  checked={connectionType === 'fb'} 
                  onChange={() => setConnectionType('fb')} 
                />
                Connect Facebook Pages only
              </label>
              <label>
                <input 
                  type="radio" 
                  value="ig" 
                  checked={connectionType === 'ig'} 
                  onChange={() => setConnectionType('ig')} 
                />
                Connect Instagram Accounts only
              </label>
              <label>
                <input 
                  type="radio" 
                  value="both" 
                  checked={connectionType === 'both'} 
                  onChange={() => setConnectionType('both')} 
                />
                Connect both
              </label>
            </div>
            <button onClick={handleFacebookLogin} className="facebook-login-btn">
              <i className="fab fa-facebook"></i> Continuar con Facebook
            </button>
          </>
        )}
      </div>
    </div>
  );
}

export default Login;