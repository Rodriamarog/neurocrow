// Login.js
import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';

function Login() {
  const navigate = useNavigate();
  const [isVerifying, setIsVerifying] = useState(false);
  const [pollInterval, setPollInterval] = useState(null);
  const [authType, setAuthType] = useState(null); // 'facebook', 'instagram', or 'facebook_business'

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
          navigate('/success', { state: { accessToken: token, authType: authType } });
        } else if (attempts >= maxAttempts) {
          clearInterval(interval);
          setIsVerifying(false);
        }
      }, true);
    }, 2000);

    setPollInterval(interval);
  };

  const handleFacebookLogin = () => {
    setAuthType('facebook');
    
    const scopeArray = [
      'pages_show_list',
      'pages_manage_metadata',
      'pages_messaging',
      'pages_read_engagement',
      'pages_manage_posts',
      'pages_manage_engagement',
      'public_profile',
      'business_management'
    ];
    
    const scope = scopeArray.join(',');
    
    console.log('🔍 Requesting Facebook-only permissions:', scope);

    window.FB.login(function(response) {
      console.log('Facebook login response:', response);
      
      if (response.status === 'connected') {
        const token = response.authResponse.accessToken;
        console.log('Successfully logged in with Facebook token');
        navigate('/success', { state: { accessToken: token, authType: 'facebook' } });
      } else if (response.status === 'not_authorized') {
        console.log('Awaiting Facebook device verification...');
        setIsVerifying(true);
        startPolling();
      } else {
        console.log('User cancelled Facebook login or did not fully authorize.');
        setIsVerifying(false);
        setAuthType(null);
      }
    }, {
      scope: scope,
      auth_type: 'rerequest'
    });
  };

  const handleInstagramLogin = () => {
    setAuthType('instagram');
    
    const instagramAppId = '1087630639166741'; // Instagram App ID
    const redirectUri = encodeURIComponent(window.location.origin + '/instagram-callback');
    const scope = encodeURIComponent('instagram_business_basic,instagram_business_manage_messages,instagram_business_manage_comments,instagram_business_content_publish,instagram_business_manage_insights');
    
    const instagramAuthUrl = `https://www.instagram.com/oauth/authorize?` +
      `force_reauth=true&` +
      `client_id=${instagramAppId}&` +
      `redirect_uri=${redirectUri}&` +
      `scope=${scope}&` +
      `response_type=code&` +
      `state=instagram_business_auth`;
    
    console.log('🔍 Redirecting to Instagram Business authorization:', instagramAuthUrl);
    
    // Redirect to Instagram Business authorization
    window.location.href = instagramAuthUrl;
  };

  const handleFacebookBusinessLogin = () => {
    setAuthType('facebook_business');
    
    const scopeArray = [
      'pages_show_list',
      'pages_manage_metadata', 
      'pages_messaging',
      'pages_read_engagement',
      'pages_manage_posts',
      'pages_manage_engagement',
      'public_profile',
      'business_management',
      'instagram_basic',
      'instagram_manage_messages',
      'instagram_manage_comments',
      'instagram_content_publish'
    ];
    
    const scope = scopeArray.join(',');
    
    console.log('🔍 Requesting Facebook Business permissions (includes Instagram):', scope);

    window.FB.login(function(response) {
      console.log('Facebook Business login response:', response);
      
      if (response.status === 'connected') {
        const token = response.authResponse.accessToken;
        console.log('Successfully logged in with Facebook Business token');
        navigate('/success', { state: { accessToken: token, authType: 'facebook_business' } });
      } else if (response.status === 'not_authorized') {
        console.log('Awaiting Facebook Business device verification...');
        setIsVerifying(true);
        startPolling();
      } else {
        console.log('User cancelled Facebook Business login or did not fully authorize.');
        setIsVerifying(false);
        setAuthType(null);
      }
    }, {
      scope: scope,
      auth_type: 'rerequest'
    });
  };

  return (
    <div className="min-h-screen bg-slate-50 dark:bg-slate-900 flex items-center justify-center p-4">
      <div className="w-full max-w-md bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 p-8 space-y-6">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100">Conecta tus cuentas</h1>
        </div>
        {isVerifying ? (
          <div className="space-y-6">
            <div className="text-center">
              <div className="w-12 h-12 mx-auto mb-4 text-slate-600">
                <i className="fas fa-spinner fa-spin text-2xl"></i>
              </div>
              <h2 className="text-xl font-semibold text-slate-900 dark:text-slate-100 mb-2">Verificación pendiente</h2>
              <p className="text-slate-600 dark:text-slate-300">Por favor, aprueba el inicio de sesión en tu aplicación de {authType === 'facebook' ? 'Facebook' : authType === 'instagram' ? 'Instagram' : 'Facebook Business'}.</p>
            </div>
            <button 
              onClick={() => {
                setIsVerifying(false);
                setAuthType(null);
                if (pollInterval) {
                  clearInterval(pollInterval);
                  setPollInterval(null);
                }
              }} 
              className="w-full px-4 py-2 text-sm font-medium text-slate-700 dark:text-slate-300 bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 hover:bg-slate-50 dark:hover:bg-slate-600 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-colors"
            >
              Cancelar
            </button>
          </div>
        ) : (
          <div className="space-y-6">
            <div className="text-center">
              <p className="text-slate-600 dark:text-slate-300">Selecciona cómo quieres conectar tus cuentas</p>
            </div>
            
            {/* Recommended Option */}
            <div className="space-y-3">
              <div className="text-center">
                <span className="inline-block px-3 py-1 text-xs font-medium bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200 rounded-full mb-2">
                  Recomendado
                </span>
              </div>
              <button 
                onClick={handleFacebookBusinessLogin} 
                className="w-full flex items-center justify-center gap-3 px-4 py-3 bg-gradient-to-r from-blue-600 to-purple-600 hover:from-blue-700 hover:to-purple-700 text-white font-medium transition-all focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
              >
                <i className="fas fa-building text-lg"></i>
                Conectar Facebook + Instagram
              </button>
              <p className="text-xs text-slate-500 dark:text-slate-400 text-center">Un solo login para ambas plataformas</p>
            </div>

            {/* Divider */}
            <div className="relative">
              <div className="absolute inset-0 flex items-center">
                <div className="w-full border-t border-slate-300 dark:border-slate-600"></div>
              </div>
              <div className="relative flex justify-center text-sm">
                <span className="px-2 bg-white dark:bg-slate-800 text-slate-500 dark:text-slate-400">o conectar individualmente</span>
              </div>
            </div>
            
            {/* Individual Options */}
            <div className="space-y-3">
              <button 
                onClick={handleFacebookLogin} 
                className="w-full flex items-center justify-center gap-3 px-4 py-3 bg-blue-600 hover:bg-blue-700 text-white font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
              >
                <i className="fab fa-facebook text-lg"></i>
                Solo Facebook
              </button>

              <button 
                onClick={handleInstagramLogin} 
                className="w-full flex items-center justify-center gap-3 px-4 py-3 bg-gradient-to-r from-purple-500 to-pink-500 hover:from-purple-600 hover:to-pink-600 text-white font-medium transition-all focus:outline-none focus:ring-2 focus:ring-purple-500 focus:ring-offset-2"
              >
                <i className="fab fa-instagram text-lg"></i>
                Solo Instagram
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export default Login;