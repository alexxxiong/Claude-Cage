import React, { useEffect, useRef, useState } from "react";
import { Download, File, Folder } from "lucide-react";
import { api, uploadFile } from "./api";
import { assertConnection, getClientContext, runtimeURL } from "./client-context.mjs";
import { Modal } from "./modal";
import "./project-files.css";

export function ProjectFiles({ folders, onClose }) {
  const [workspace, setWorkspace] = useState(folders[0]);
  const [parts, setParts] = useState([]);
  const [files, setFiles] = useState([]);
  const [revision, refresh] = useState(0);
  const [name, setName] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const picker = useRef(null);
  const connection = useRef(getClientContext().connection_id).current;
  const [listError, setListError] = useState("");
  const path = parts.join("/") || ".";
  const endpoint = "/projects/files?" + new URLSearchParams({ workspace, path });
  const operation = (action, filename) => endpoint + "&" + new URLSearchParams({ action, name: filename });
  useEffect(() => {
    const abort = new AbortController();
    setFiles([]); setLoading(true); setListError("");
    api(endpoint, undefined, "GET", {}, { signal: abort.signal })
      .then(result => { if (!abort.signal.aborted) setFiles(result.files); })
      .catch(e => { if (!abort.signal.aborted) setListError(e.message); })
      .finally(() => { if (!abort.signal.aborted) setLoading(false); });
    return () => abort.abort();
  }, [endpoint, revision]);
  const run = async (label, task) => {
    if (busy) return;
    setBusy(label); setError(""); setNotice("");
    try { assertConnection(connection); await task(); }
    catch (e) { if (e.name !== "AbortError") setError(e.message); }
    finally { setBusy(""); }
  };
  const upload = selected => run("上传中", async () => {
    let count = 0;
    try {
      for (const file of selected) {
        assertConnection(connection);
        await uploadFile(operation("upload", file.name), file, {
          onProgress: (done, total) => setBusy(file.name + " · " + Math.round(100 * done / Math.max(total, 1)) + "%"),
        });
        count++;
      }
    } finally {
      setNotice("已上传 " + count + " 个文件");
      refresh(n => n + 1);
    }
  });
  const download = entry => run("下载中", async () => {
    const target = await window.showSaveFilePicker({ suggestedName: entry.name });
    assertConnection(connection);
    const response = await fetch(runtimeURL(operation("download", entry.name)));
    if (!response.ok) throw new Error("下载失败 (" + response.status + ")");
    const stream = await target.createWritable();
    await response.body.pipeTo(stream);
    setNotice("已保存 " + entry.name);
  });
  return <Modal title="项目文件" wide onClose={busy ? undefined : onClose}>
    <div className="cage-project-files" onDragOver={e => e.preventDefault()} onDrop={e => {
      e.preventDefault(); if (!busy && !loading && e.dataTransfer.files.length) upload([...e.dataTransfer.files]);
    }}>
      <div className="cpf-path">{folders.length > 1 ? <select disabled={!!busy} value={workspace} onChange={e => { setWorkspace(e.target.value); setParts([]); }}>{folders.map(f => <option key={f}>{f}</option>)}</select> : workspace}{parts.length > 0 && "/" + parts.join("/")}</div>
      <div className="cpf-actions"><button disabled={!parts.length || !!busy} onClick={() => setParts(parts.slice(0,-1))}>上一级</button>
      <button disabled={!!busy} onClick={() => setCreating(!creating)}>新建文件夹</button>
      <button disabled={!!busy || loading} onClick={() => picker.current.click()}>上传文件…</button>
      <button disabled={!!busy} onClick={() => refresh(n => n+1)}>刷新</button>
      <input ref={picker} type="file" multiple hidden onChange={e => { const selected = [...e.target.files]; e.target.value = ""; if (selected.length) upload(selected); }}/></div>
      {creating && <form className="cpf-actions" onSubmit={e => { e.preventDefault(); run("正在创建", async () => {
        await api(operation("mkdir",name),{}); setName(""); setCreating(false); refresh(n => n+1);
      }); }}><input aria-label="文件夹名称" placeholder="文件夹名称" autoFocus value={name} onChange={e => setName(e.target.value)}/><button disabled={!!busy || !name.trim()}>创建</button></form>}
      <div className="cpf-list">{loading ? "读取中…" : !files.length && !listError ? "空文件夹，可拖入文件" : files.map(item => <div className="cpf-row" key={item.name}>
        {item.dir ? <button disabled={!!busy} onClick={() => {setParts([...parts,item.name]);setNotice("");}}><Folder size={16}/>{item.name}</button> : <><span><File size={16}/>{item.name}</span><small>{item.size.toLocaleString()} B</small><button disabled={!!busy} aria-label={"下载 "+item.name} onClick={() => download(item)}><Download size={16}/>下载</button></>}
      </div>)}</div>
      <div role="status">{busy || notice}</div>{(error || listError) && <div role="alert" className="error-message">{error || listError}</div>}
    </div>
  </Modal>;
}
