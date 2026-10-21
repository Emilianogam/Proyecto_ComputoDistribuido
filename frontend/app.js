// El frontend solo conoce al proxy de nginx. Nunca llama a un servicio directo.
const PROXY = "http://localhost:8000";

// Los tres servicios que estan registrados en Consul.
const SERVICIOS = ["categorias", "tareas", "usuarios"];

// peticion hace el fetch y regresa los datos junto con la instancia
// que respondio, que viene en el header X-Instancia.
async function peticion(ruta, opciones = {}) {
  const resp = await fetch(PROXY + ruta, {
    headers: { "Content-Type": "application/json" },
    ...opciones,
  });

  const instancia = resp.headers.get("X-Instancia") || "?";

  if (resp.status === 204) {
    return { datos: null, instancia };
  }
  if (!resp.ok) {
    throw new Error("nginx respondio " + resp.status);
  }
  return { datos: await resp.json(), instancia };
}

function mostrarInstancia(servicio, instancia) {
  document.getElementById(servicio + "-instancia").textContent =
    "Respondio: " + instancia;
}

function escribirBitacora(texto) {
  const b = document.getElementById("bitacora");
  b.textContent = texto + "\n" + b.textContent;
}

// ---------- Registro de servicios ----------
// cargarServicios le pregunta a Consul, a traves de nginx, que instancias
// hay registradas y cual es su estado de salud. Antes esto salia de un
// archivo estatico; ahora sale del registro dinamico.

async function cargarServicios() {
  const contenedor = document.getElementById("servicios");
  contenedor.innerHTML = "";

  for (const servicio of SERVICIOS) {
    const resp = await fetch(PROXY + "/consul/v1/health/service/" + servicio);
    const entradas = await resp.json();

    entradas.sort((a, b) => a.Service.ID.localeCompare(b.Service.ID));

    const div = document.createElement("div");
    const titulo = document.createElement("strong");
    titulo.textContent = servicio;
    div.appendChild(titulo);

    const ul = document.createElement("ul");

    if (entradas.length === 0) {
      const li = document.createElement("li");
      li.className = "caida";
      li.textContent = "sin instancias registradas";
      ul.appendChild(li);
    }

    for (const entrada of entradas) {
      // Consul marca "passing" cuando el health check responde bien.
      const viva = entrada.Checks.every((c) => c.Status === "passing");

      const li = document.createElement("li");
      li.className = viva ? "viva" : "caida";
      li.textContent =
        entrada.Service.ID + " (" + entrada.Service.Address + ":" + entrada.Service.Port + ")";
      ul.appendChild(li);
    }

    div.appendChild(ul);
    contenedor.appendChild(div);
  }
}

// probarBalanceo manda 9 GET seguidos a /usuarios para que se vea
// como el round robin va rotando entre las tres instancias.
async function probarBalanceo() {
  document.getElementById("bitacora").textContent = "";
  for (let i = 1; i <= 9; i++) {
    const { instancia } = await peticion("/usuarios");
    escribirBitacora("peticion " + i + " -> " + instancia);
  }
}

// cargarResumen le pide el resumen al servicio de tareas. Para contestarlo,
// tareas llama por gRPC a usuarios y a categorias pasando por el balanceador.
// Por eso el resultado dice que instancia de cada servicio contesto.
async function cargarResumen() {
  const resp = await fetch(PROXY + "/tareas/resumen");
  const r = await resp.json();

  document.getElementById("bitacora").textContent =
    "resumen armado con llamadas gRPC entre servicios\n" +
    "usuarios:   " + r.usuarios + "   (respondio " + r.instancia_usuarios + ")\n" +
    "tareas:     " + r.tareas + "   (lo armo " + r.instancia + ")\n" +
    "categorias: " + r.categorias + "   (respondio " + r.instancia_categorias + ")";
}

// ---------- Usuarios ----------

async function cargarUsuarios() {
  const { datos, instancia } = await peticion("/usuarios");
  mostrarInstancia("usuarios", instancia);

  const ul = document.getElementById("usuarios-lista");
  ul.innerHTML = "";
  for (const u of datos) {
    const li = document.createElement("li");
    li.textContent = u.nombre + (u.email ? " (" + u.email + ")" : "");
    const btn = document.createElement("button");
    btn.textContent = "Borrar";
    btn.onclick = () => borrarUsuario(u.id);
    li.appendChild(btn);
    ul.appendChild(li);
  }
}

async function crearUsuario(evento) {
  evento.preventDefault();
  const nombre = document.getElementById("usuario-nombre");
  const email = document.getElementById("usuario-email");

  const { instancia } = await peticion("/usuarios", {
    method: "POST",
    body: JSON.stringify({ nombre: nombre.value, email: email.value }),
  });

  escribirBitacora("POST /usuarios -> " + instancia);
  nombre.value = "";
  email.value = "";
  cargarUsuarios();
}

async function borrarUsuario(id) {
  const { instancia } = await peticion("/usuarios/" + id, { method: "DELETE" });
  escribirBitacora("DELETE /usuarios/" + id + " -> " + instancia);
  cargarUsuarios();
}

// ---------- Tareas ----------

async function cargarTareas() {
  const { datos, instancia } = await peticion("/tareas");
  mostrarInstancia("tareas", instancia);

  const ul = document.getElementById("tareas-lista");
  ul.innerHTML = "";
  for (const t of datos) {
    const li = document.createElement("li");
    li.textContent = t.titulo + (t.completada ? " [hecha]" : "");

    const acciones = document.createElement("span");

    const btnCompletar = document.createElement("button");
    btnCompletar.textContent = t.completada ? "Reabrir" : "Completar";
    btnCompletar.onclick = () => completarTarea(t);
    acciones.appendChild(btnCompletar);

    const btnBorrar = document.createElement("button");
    btnBorrar.textContent = "Borrar";
    btnBorrar.onclick = () => borrarTarea(t.id);
    acciones.appendChild(btnBorrar);

    li.appendChild(acciones);
    ul.appendChild(li);
  }
}

async function crearTarea(evento) {
  evento.preventDefault();
  const titulo = document.getElementById("tarea-titulo");

  const { instancia } = await peticion("/tareas", {
    method: "POST",
    body: JSON.stringify({ titulo: titulo.value, completada: false }),
  });

  escribirBitacora("POST /tareas -> " + instancia);
  titulo.value = "";
  cargarTareas();
}

async function completarTarea(tarea) {
  const { instancia } = await peticion("/tareas/" + tarea.id, {
    method: "PUT",
    body: JSON.stringify({ titulo: tarea.titulo, completada: !tarea.completada }),
  });
  escribirBitacora("PUT /tareas/" + tarea.id + " -> " + instancia);
  cargarTareas();
}

async function borrarTarea(id) {
  const { instancia } = await peticion("/tareas/" + id, { method: "DELETE" });
  escribirBitacora("DELETE /tareas/" + id + " -> " + instancia);
  cargarTareas();
}

// ---------- Categorias ----------

async function cargarCategorias() {
  const { datos, instancia } = await peticion("/categorias");
  mostrarInstancia("categorias", instancia);

  const ul = document.getElementById("categorias-lista");
  ul.innerHTML = "";
  for (const c of datos) {
    const li = document.createElement("li");
    li.textContent = c.nombre + (c.color ? " (" + c.color + ")" : "");
    const btn = document.createElement("button");
    btn.textContent = "Borrar";
    btn.onclick = () => borrarCategoria(c.id);
    li.appendChild(btn);
    ul.appendChild(li);
  }
}

async function crearCategoria(evento) {
  evento.preventDefault();
  const nombre = document.getElementById("categoria-nombre");
  const color = document.getElementById("categoria-color");

  const { instancia } = await peticion("/categorias", {
    method: "POST",
    body: JSON.stringify({ nombre: nombre.value, color: color.value }),
  });

  escribirBitacora("POST /categorias -> " + instancia);
  nombre.value = "";
  color.value = "";
  cargarCategorias();
}

async function borrarCategoria(id) {
  const { instancia } = await peticion("/categorias/" + id, { method: "DELETE" });
  escribirBitacora("DELETE /categorias/" + id + " -> " + instancia);
  cargarCategorias();
}

// ---------- Arranque ----------

cargarServicios();
cargarUsuarios();
cargarTareas();
cargarCategorias();
