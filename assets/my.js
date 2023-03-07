let timeout = 5000 // milliseconds
let separator = '/';
let tree = [];
let idCount = 0;
let editor = ace.edit('value');
editor.$blockScrolling = Infinity;


$(document).ready(function () {
    $('#etree').tree({
        animate: true,
        onClick: showNode,
        onContextMenu: showMenu
    });
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
        },
        error: function (err) {
            $.messager.alert('Error', $.toJSON(err), 'error');
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
    editor.setReadOnly(false);
    $('#footer').html('&nbsp;');
}

function showNode(node) {
    $('#elayout').layout('panel', 'center').panel('setTitle', node.path);
    editor.getSession().setValue('');
    if (node.dir === false) {
        editor.setReadOnly(false);
        $.ajax({
            type: 'GET',
            timeout: timeout,
            url: '/v3/get',
            data: {'key': node.path},
            async: true,
            dataType: 'json',
            success: function (data) {
                if (data.errorCode) {
                    $('#etree').tree('remove', node.target);
                    console.log(data.message);
                    resetValue()
                } else {
                    editor.getSession().setValue(data.node.value);
                    var ttl = 0;
                    if (data.node.ttl) {
                        ttl = data.node.ttl;
                    }
                    changeFooter(ttl, data.node.createdIndex, data.node.modifiedIndex);
                }
            },
            error: function (err) {
                $.messager.alert('Error', $.toJSON(err), 'error');
            }
        });
    } else {
        if (node.children.length > 0) {
            $('#etree').tree(node.state === 'closed' ? 'expand' : 'collapse', node.target);
        }
        $('#footer').html('&nbsp;');
        // clear child node
        var children = $('#etree').tree('getChildren', node.target);
        var url = '';
        url = '/v3/getpath';
        $.ajax({
            type: 'GET',
            timeout: timeout,
            url: url,
            data: {'key': node.path, 'prefix': 'true'},
            async: true,
            dataType: 'json',
            success: function (data) {
                if (data.node.value) {
                    editor.getSession().setValue(data.node.value);
                    changeFooter(data.node.ttl, data.node.createdIndex, data.node.modifiedIndex);
                }
                let arr = [];

                if (data.node.nodes) {
                    // refresh child node
                    for (var i in data.node.nodes) {
                        var newData = getNode(data.node.nodes[i]);
                        arr.push(newData);
                    }
                    $('#etree').tree('append', {
                        parent: node.target,
                        data: arr
                    });
                }

                for (var n in children) {
                    $('#etree').tree('remove', children[n].target);
                }

            },
            error: function (err) {
                $.messager.alert('Error', $.toJSON(err), 'error');
            }
        });
    }
}

function getNode(n) {
    var path = n.key.split(separator);
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
            for (var i in n.nodes) {
                var rn = getNode(n.nodes[i]);
                obj.children.push(rn);
            }
        }
    }
    return obj
}

function showMenu(e, node) {
    e.preventDefault();
    $('#etree').tree('select', node.target);
    $('#treeDirMenu').menu('show', {
        left: e.pageX,
        top: e.pageY
    });
}


function getId() {
    return idCount++;
}


function changeFooter(ttl, cIndex, mIndex) {
    $('#footer').html('<span>TTL&nbsp;:&nbsp;' + ttl + '&nbsp;&nbsp;&nbsp;&nbsp;CreateRevision&nbsp;:&nbsp;' + cIndex + '&nbsp;&nbsp;&nbsp;&nbsp;ModRevision&nbsp;:&nbsp;' + mIndex + '</span><span id="showMode" style="position: absolute;right: 10px;color: #777;">' + '</span>');
}