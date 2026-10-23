package usecase

import (
	"strings"

	"github.com/mikbil/mikbil/internal/domain"
)

// HotspotUsecase menyusun template login.html dan snippet tombol beli.
type HotspotUsecase struct {
	set SettingsRepository
}

// NewHotspotUsecase membuat HotspotUsecase.
func NewHotspotUsecase(set SettingsRepository) *HotspotUsecase {
	return &HotspotUsecase{set: set}
}

func (u *HotspotUsecase) siteURL() (string, error) {
	siteURL, found, err := u.set.Get(domain.SettingSiteURL)
	if err != nil {
		return "", err
	}
	if !found || strings.TrimSpace(siteURL) == "" {
		return "", InputError{"alamat situs belum diatur"}
	}
	return strings.TrimRight(strings.TrimSpace(siteURL), "/"), nil
}

// ButtonSnippet mengembalikan potongan HTML tombol "Beli Voucher" siap tempel.
func (u *HotspotUsecase) ButtonSnippet() (string, error) {
	siteURL, err := u.siteURL()
	if err != nil {
		return "", err
	}
	return `<!-- Mikbil: tombol beli voucher (tempel di dalam <body> login.html) -->
<a href="` + siteURL + `/?mac=$(mac)&amp;ip=$(ip)"
   style="display:block;text-align:center;padding:14px;border-radius:10px;
          background:#16a34a;color:#fff;font-weight:700;text-decoration:none;">
  Beli Voucher WiFi
</a>
`, nil
}

// LoginTemplate mengembalikan isi login.html siap unduh (self-contained).
func (u *HotspotUsecase) LoginTemplate() (string, error) {
	siteURL, err := u.siteURL()
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(loginTemplateHTML, "__MIKBIL_URL__", siteURL), nil
}

// loginTemplateHTML adalah template login hotspot: CSS dan JS inline penuh,
// sistem font, CHAP dengan fallback PAP, form voucher, tombol beli Mikbil.
const loginTemplateHTML = `<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Login WiFi</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",system-ui,sans-serif;background:#09090b;color:#fafafa;min-height:100vh;display:flex;align-items:center;justify-content:center;padding:20px}
.card{width:100%;max-width:420px;background:#18181b;border:1px solid #27272a;border-radius:16px;padding:28px 24px}
.brand{display:flex;align-items:center;gap:10px;margin-bottom:6px}
.brand-mark{width:38px;height:38px;border-radius:10px;background:#10b981;color:#052e16;display:flex;align-items:center;justify-content:center;font-weight:800;font-size:20px}
h1{font-size:22px;letter-spacing:-.02em}
.sub{color:#a1a1aa;font-size:14px;margin-bottom:18px}
label{display:block;font-weight:600;font-size:14px;margin:12px 0 6px}
input{width:100%;height:48px;border-radius:10px;border:1px solid #3f3f46;background:#09090b;color:#fafafa;font-size:16px;padding:0 14px}
input:focus{outline:2px solid #10b981;outline-offset:1px}
button{width:100%;height:48px;border-radius:10px;border:0;background:#10b981;color:#052e16;font-size:16px;font-weight:700;cursor:pointer;margin-top:14px}
button:active{transform:scale(.98)}
.divider{display:flex;align-items:center;gap:10px;color:#a1a1aa;font-size:13px;margin:18px 0}
.divider::before,.divider::after{content:"";flex:1;height:1px;background:#27272a}
.buy{display:block;text-align:center;padding:14px;border-radius:10px;background:#16a34a;color:#fff;font-weight:700;text-decoration:none;font-size:16px}
.buy:active{transform:scale(.98)}
.error{background:#450a0a;border:1px solid #7f1d1d;color:#fecaca;border-radius:10px;padding:10px 14px;font-size:14px;margin-bottom:14px}
.trial{display:block;text-align:center;margin-top:12px;color:#a1a1aa;font-size:14px}
.note{margin-top:14px;color:#a1a1aa;font-size:13px;text-align:center}
</style>
</head>
<body>
<div class="card">
  <div class="brand"><span class="brand-mark">W</span><h1>Login WiFi</h1></div>
  <p class="sub">Masuk dengan akun atau kode voucher Anda.</p>
  $(if error)<div class="error">$(error)</div>$(endif)
  <form name="sendin" action="$(link-login-only)" method="post" style="display:none">
    <input type="hidden" name="username">
    <input type="hidden" name="password">
    <input type="hidden" name="dst" value="$(link-orig)">
  </form>
  <form name="login" onsubmit="return doLogin()">
    <label for="username">Username</label>
    <input id="username" name="username" type="text" value="$(username)" autocomplete="username" required>
    <label for="password">Password</label>
    <input id="password" name="password" type="password" autocomplete="current-password" required>
    <button type="submit">Masuk</button>
  </form>
  <div class="divider">atau pakai kode voucher</div>
  <form name="voucher" onsubmit="return doVoucher()">
    <label for="vcode">Kode voucher</label>
    <input id="vcode" name="vcode" type="text" autocomplete="off" placeholder="cth. K7pQ2xTm" required>
    <button type="submit">Tukar Voucher</button>
  </form>
  <div class="divider">belum punya voucher</div>
  <a class="buy" href="__MIKBIL_URL__/?mac=$(mac)&amp;ip=$(ip)">Beli Voucher WiFi</a>
  $(if trial)<a class="trial" href="$(link-login-only)?dst=$(link-orig)&amp;username=T-$(mac-esc)">Coba gratis</a>$(endif)
  <p class="note">Butuh bantuan? Hubungi pemilik hotspot.</p>
</div>
<script>
function md5hex(s){
  function rl(n,c){return (n<<c)|(n>>>32-c);}
  var T=[];for(var i=0;i<64;i++){T[i]=Math.floor(Math.abs(Math.sin(i+1))*4294967296);}
  var b=[];for(var j=0;j<s.length;j++){b.push(s.charCodeAt(j)&255);}
  var bitlen=b.length*8;b.push(128);
  while(b.length%64!==56){b.push(0);}
  for(var k=0;k<8;k++){b.push(Math.floor(bitlen/Math.pow(256,k))%256);}
  var a0=1732584193,a1=4023233417,a2=2562383102,a3=271733878;
  var S=[7,12,17,22,5,9,14,20,4,11,16,23,6,10,15,21];
  for(var o=0;o<b.length;o+=64){
    var M=[];for(var m=0;m<16;m++){M[m]=b[o+m*4]|(b[o+m*4+1]<<8)|(b[o+m*4+2]<<16)|(b[o+m*4+3]<<24);}
    var A=a0,B=a1,C=a2,D=a3;
    for(var r=0;r<64;r++){
      var F,g;
      if(r<16){F=(B&C)|(~B&D);g=r;}
      else if(r<32){F=(D&B)|(~D&C);g=(5*r+1)%16;}
      else if(r<48){F=B^C^D;g=(3*r+5)%16;}
      else{F=C^(B|~D);g=(7*r)%16;}
      F=(F+A+T[r]+M[g])|0;A=D;D=C;C=B;
      B=(B+rl(F,S[(r>>4)*4+(r%4)]))|0;
    }
    a0=(a0+A)|0;a1=(a1+B)|0;a2=(a2+C)|0;a3=(a3+D)|0;
  }
  function hex(n){var s2="";for(var q=0;q<4;q++){s2+=("0"+((n>>(q*8))&255).toString(16)).slice(-2);}return s2;}
  return hex(a0)+hex(a1)+hex(a2)+hex(a3);
}
function submitLogin(user, pass){
  document.sendin.username.value=user;
  var chapId="$(chap-id)";
  if(chapId!==""&&chapId!=="$("+"chap-id)"){
    document.sendin.password.value=md5hex(chapId+pass+"$(chap-challenge)");
  }else{
    document.sendin.password.value=pass;
  }
  document.sendin.submit();
  return false;
}
function doLogin(){
  return submitLogin(document.login.username.value, document.login.password.value);
}
function doVoucher(){
  var code=document.voucher.vcode.value;
  return submitLogin(code, code);
}
</script>
</body>
</html>
`
