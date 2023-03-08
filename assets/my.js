let timeout = 5000 // milliseconds
let separator = '/';
let tree = [];
let idCount = 0;
let editor = ace.edit('value');
editor.$blockScrolling = Infinity;


$(document).ready(function () {
    $('#etree').tree({animate: true, onClick: showNode});
    connect();
});

function connect() {
    $.ajax({
        type: 'GET',
        timeout: timeout,
        url: '/v3/connect',
        async: false,
        dataType: 'json',
        success: function (data) {
            $('#statusVersion').html('ETCD version:' + data.info.version);
            $('#statusSize').html('Size:' + data.info.size)
            $('#statusMember').html('Member name:' + data.info.name)
        }
    });
    reload();
}

function reload() {
    const rootNode = {
        id: getId(),
        children: [],
        dir: true,
        path: separator,
        text: separator,
        iconCls: 'icon-dir'
    };
    tree = [];
    tree.push(rootNode);
    $('#etree').tree('loadData', tree);
    showNode($('#etree').tree('getRoot'));
    resetValue();
}

function resetValue() {
    $('#elayout').layout('panel', 'center').panel('setTitle', separator);
    editor.getSession().setValue('');
    $('#footer').html('&nbsp;');
}

function format() {
    let val = JSON.parse(editor.getValue());
    editor.setValue(JSON.stringify(val, null, 4));
    editor.getSession().setMode('ace/mode/json');
    editor.clearSelection();
    editor.navigateFileStart();
}

function getpath(node) {
    if (node.children.length > 0) {
        $('#etree').tree(node.state === 'closed' ? 'expand' : 'collapse', node.target);
    }
    $('#footer').html('&nbsp;');
    let children = $('#etree').tree('getChildren', node.target);
    $.ajax({
        type: 'GET',
        timeout: timeout,
        url: '/v3/getpath',
        data: {'key': node.path, 'prefix': 'true'},
        async: true,
        dataType: 'json',
        success: function (data) {
            let arr = [];
            if (data.node.nodes) {
                for (let i in data.node.nodes) {
                    let newData = getNode(data.node.nodes[i]);
                    arr.push(newData);
                }
                $('#etree').tree('append', {parent: node.target, data: arr});
            }
            for (let n in children) {
                $('#etree').tree('remove', children[n].target);
            }
        }
    });
}

function get(node) {
    $.ajax({
            type: 'GET',
            timeout: timeout,
            url: '/v3/get',
            data: {'key': node.path},
            async: true,
            dataType: 'json',
            success: function (data) {
                editor.getSession().setValue(data.node.value);
                format()
                changeFooter(data.node.ttl, data.node.createdIndex, data.node.modifiedIndex);
            }
        }
    );
}

function showNode(node) {
    $('#elayout').layout('panel', 'center').panel('setTitle', node.path);
    editor.getSession().setValue('');
    if (node.dir === false) {
        get(node);
    } else {
        getpath(node);
    }
}

function getNode(n) {
    let path = n.key.split(separator);
    let text = path[path.length - 1];
    const obj = {
        id: getId(),
        text: text,
        dir: false,
        iconCls: 'icon-text',
        path: n.key,
        children: []
    };
    if (n.dir === true) {
        obj.state = 'closed';
        obj.dir = true;
        obj.iconCls = 'icon-dir';
        if (n.nodes) {
            for (let i in n.nodes) {
                let rn = getNode(n.nodes[i]);
                obj.children.push(rn);
            }
        }
    }
    return obj
}


function getId() {
    return idCount++;
}


function changeFooter(ttl, cIndex, mIndex) {
    $('#footer').html('<span>TTL&nbsp;:&nbsp;' + ttl + '&nbsp;&nbsp;&nbsp;&nbsp;CreateRevision&nbsp;:&nbsp;' + cIndex + '&nbsp;&nbsp;&nbsp;&nbsp;ModRevision&nbsp;:&nbsp;' + mIndex + '</span><span id="showMode" style="position: absolute;right: 10px;color: #777;">' + '</span>');
}