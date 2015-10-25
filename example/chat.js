
var nic
var WSChatroom

$(document).ready(function() {
    // Connexion to websocket
    var dsn = "wss://";
    if (window.location.protocol == "http:") {
        dsn = "ws://";
    }
    WSChatroom = new WebSocket(dsn + window.location.host + '/ws');
    if (!WSChatroom) {
        alert("Sorry but your browser doesn't support websocket. Please use <a href='https://www.google.com/intl/en/chrome/browser/' target='_blank'>Google Chrome</a> or <a href='http://www.mozilla.org/en-US/firefox/new/' target='_blank'> Firefox </a>");
        return;
    }

    // open
    WSChatroom.onopen = function (event) {
        // log
        console.log("websocket opened");
    };

    // close
    WSChatroom.onclose = function (event) {
        alert("Websocket lost. Reload current page")
    }

    // error
    WSChatroom.onerror = function (event) {
        console.log(event);
    }

    // Message
    WSChatroom.onmessage = function (event) {
        // heartbeat
        if (event.data=="p"){
            WSChatroom.send("p");
            return;
        }
        msg = JSON.parse(event.data);
        console.log(msg);
        switch(msg.cmd) {
            case "error":
                alert(msg.data)
                break;
            case "joinOK":
                // Show chat panel
                $("#chatLogin").addClass("hidden");
                $("#ulmessages").empty();
                $("#chatMessages").removeClass("hidden");
                $("#chatNewmsg").removeClass("hidden");
                $("#message").focus();
                break;
            case "newchatmsg":
		var t =  new Date(msg.timestamp*1000);
		var toAdd = `<li `;
		if (msg.user=="chatbot"){
			toAdd+=` class="text-muted"`;
		}
		toAdd = toAdd+"><small>"+t.toLocaleString()+" </small>";
		if (msg.user!="chatbot"){
			toAdd+="<strong>"+msg.user+"</strong> ";
		}
		toAdd+=msg.data + "</li>";

		$("#ulmessages").append(toAdd);
                // scroll down
                $("#chatMessages").animate({scrollTop: $("#chatMessages")[0].scrollHeight}, 0);
                $("#soundNewMessage").trigger('play');
                break
            default :
                console.log("unexpected message from ws");
                console.log(msg)
        }

    }

    // Enter chatroom
    var enterChatroom = function () {
        nic = $("#nic").val().trim();
        if (nic == "") {
            return
        }
        var msg = {
            cmd: "join",
            data: `{"nic":"` + nic+`","room":"test"}`
        }
        WSChatroom.send(JSON.stringify(msg));
    }


    $("#btnEnterChatroom").click(enterChatroom);
    $("#nic").keyup(function (event) {
        if (event.keyCode == 13) {
            enterChatroom();
        }
    })

    // Send message
    var sendMessage = function () {
        msg = $("#message").val().trim();
        $("#message").val("");
        if (msg == "") {
            return;
        }
        WSChatroom.send(JSON.stringify({
            cmd: "newmsg",
            data: msg
        }));
    }

    $("#btnSendMsg").click(sendMessage);
    $("#message").keyup(function (event) {
        if (event.keyCode == 13) {
            sendMessage();
        }
    })
});
