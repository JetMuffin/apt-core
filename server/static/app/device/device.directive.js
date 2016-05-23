'use strict';

angular.module('aptWebApp')
    .directive("drawScreen", function($window, $http) {
        return {
            restrict: "A",
            link: function(scope, element, attributes) {
                var BLANK_IMG = 'data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEKAAEALAAAAAABAAEAAAICTAEAOw==';
                scope.screenCanvas = element[0];
                scope.screenG = scope.screenCanvas.getContext('2d');

                var deviceID = attributes['devid'];
                $http.get('device/ip/' + deviceID).then(function(response) {
                    if (response) {
                        var nodeIP = response.data;
                        //console.log(nodeIP)
                        var wsUrl = 'ws://' + nodeIP + ':9002/' + deviceID;
                        var ws = new WebSocket(wsUrl);
                        scope.screenWS = ws;
                        scope.logs = [];
                        //ws.binaryType = 'blob';

                        ws.onclose = function() {
                            console.log('onclose');
                        };
                        //screen display
                        ws.onerror = function() {
                            console.log('onerror');
                        };

                        ws.onmessage = function(message) {
                            if (typeof message.data == 'string') {
                                //log content
                                var segments = message.data.split(' ');
                                var log = {};
                                if (segments.length < 4) {
                                    log.kind = '';
                                    log.date = '';
                                    log.content = message.data;
                                    scope.logs.push(log);
                                } else {
                                    log.kind = segments[2][0];
                                    log.date = segments[0] +' '+ segments[1];
                                    log.content = '';
                                    for (var i = 2; i < segments.length; i++) {
                                        log.content = log.content + ' ' + segments[i]
                                    }
                                    scope.logs.push(log);
                                    console.log(log);
                                }
                            } else {
                                //image binary
                                var blob = new Blob([message.data], {
                                    type: 'image/jpeg'
                                });
                                var URL = window.URL || window.webkitURL
                                var img = new Image();
                                img.onload = function() {
                                    //console.log(img.width, img.height)
                                    canvas.width = img.width
                                    canvas.height = img.height
                                    g.drawImage(img, 0, 0)
                                    img.onload = null
                                    img.src = BLANK_IMG
                                    img = null
                                    u = null
                                    blob = null
                                };
                                var u = URL.createObjectURL(blob);
                                img.src = u;
                            }
                        };

                        ws.onopen = function() {
                            console.log('onopen', arguments);
                            ws.send('1920x1080/0');
                        };

                        //UI events
                        var drawing = false;
                        var lastX;
                        var lastY;
                        element.bind('mousedown', function(event) {
                            if (event.offsetX !== undefined) {
                                lastX = event.offsetX;
                                lastY = event.offsetY;
                            } else {
                                lastX = event.layerX - event.currentTarget.offsetLeft;
                                lastY = event.layerY - event.currentTarget.offsetTop;
                            }
                            drawing = true;
                        });
                        // element.bind('mousemove', function(event) {
                        //     console.log(event);
                        // });
                        element.bind('mouseup', function(event) {
                            if (drawing) {
                                var currentX;
                                var currentY;
                                // get current mouse position
                                if (event.offsetX !== undefined) {
                                    currentX = event.offsetX;
                                    currentY = event.offsetY;
                                } else {
                                    currentX = event.layerX - event.currentTarget.offsetLeft;
                                    currentY = event.layerY - event.currentTarget.offsetTop;
                                }
                                sendEvent(lastX, lastY, currentX, currentY);
                                drawing = false;
                            }
                        });
                        //send UI event
                        function sendEvent(x1, y1, x2, y2) {
                            var dist = (x1 - x2) * (x1 - x2) + (y1 - y2) * (y1 - y2)
                            if (dist < 4) {
                                var evt = x1 + ',' + y1;
                                ws.send(evt)
                            } else {
                                var evt = x1 + ',' + y1 + ',' + x2 + ',' + y2;
                                ws.send(evt);
                            }
                        }

                        scope.$on('$destroy', function() {
                            ws.close();
                        });
                    }
                }, function(response) {

                });
            }
        };
    });
