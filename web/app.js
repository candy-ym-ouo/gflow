fetch('/api/v1/healthz').then(r=>r.json()).then(x=>document.querySelector('#health').textContent=JSON.stringify(x,null,2));
