let timeout = 5000,
    separator = '/',
    tree = [],
    idCount = 0,
    editor = ace.edit('value');
editor.$blockScrolling = Infinity;

$(document).ready(() => {
    $('#etree').tree({
        animate: true,
        onClick: showNode,
    });
    connect();
});

function connect() {
    const rootNode = {id: getId(), children: [], dir: true, path: separator, text: separator, iconCls: 'icon-dir'};
    tree = [rootNode];
    const $etree = $('#etree');
    const $elayout = $('#elayout');
    $etree.tree('loadData', tree);
    showNode($etree.tree('getRoot'));
    $elayout.layout('panel', 'center').panel('setTitle', separator);
    editor.getSession().setValue('');
}


function format() {
    let val = JSON.parse(editor.getValue());
    editor.setValue(JSON.stringify(val, null, 4));
    editor.getSession().setMode('ace/mode/json');
    editor.clearSelection();
    editor.navigateFileStart();
}

function getpath(node) {
    if (node.children.length > 0) $('#etree').tree(node.state === 'closed' ? 'expand' : 'collapse', node.target);
    $('#footer').html('&nbsp;');
    let children = $('#etree').tree('getChildren', node.target);
    $.ajax({
        type: 'GET',
        timeout,
        url: '/v3/getpath',
        data: {'key': node.path, 'prefix': 'true'},
        async: true,
        dataType: 'json'
    })
        .done(data => {
            if (data.node.nodes) {
                let arr = data.node.nodes.map(getNode);
                $('#etree').tree('append', {parent: node.target, data: arr});
            }
            children.forEach(n => $('#etree').tree('remove', n.target));
        });
}

function get(node) {
    $.ajax({type: 'GET', timeout, url: '/v3/get', data: {'key': node.path}, async: true, dataType: 'json'})
        .done(data => {
            editor.getSession().setValue(data.node.value);
            format();
        });
}

function showNode(node) {
    $('#elayout').layout('panel', 'center').panel('setTitle', node.path);
    editor.getSession().setValue('');
    node.dir === false ? get(node) : getpath(node);
}

function getNode(n) {
    let path = n.key.split(separator), text = path[path.length - 1];
    const obj = {id: getId(), text, dir: false, iconCls: 'icon-text', path: n.key, children: []};
    if (n.dir) {
        obj.state = 'closed';
        obj.dir = true;
        obj.iconCls = 'icon-dir';
        if (n.nodes) obj.children = n.nodes.map(getNode);
    }
    return obj;
}


function getId() {
    return idCount++;
}


