/*!
 * jQuery Cookie Plugin v1.4.1
 * https://github.com/carhartl/jquery-cookie
 *
 * Copyright 2013 Klaus Hartl
 * Released under the MIT license
 */
!function(e){"function"==typeof define&&define.amd?define(["jquery"],e):e("object"==typeof exports?require("jquery"):jQuery)}(function(e){var i=/\+/g;function o(e){return t.raw?e:encodeURIComponent(e)}function r(e){return t.raw?e:decodeURIComponent(e)}function n(o,r){var n=t.raw?o:function e(o){0===o.indexOf('"')&&(o=o.slice(1,-1).replace(/\\"/g,'"').replace(/\\\\/g,"\\"));try{return o=decodeURIComponent(o.replace(i," ")),t.json?JSON.parse(o):o}catch(r){}}(o);return e.isFunction(r)?r(n):n}var t=e.cookie=function(i,c,u){if(void 0!==c&&!e.isFunction(c)){if("number"==typeof(u=e.extend({},t.defaults,u)).expires){var a,s=u.expires,f=u.expires=new Date;f.setTime(+f+864e5*s)}return document.cookie=[o(i),"=",(a=c,o(t.json?JSON.stringify(a):String(a))),u.expires?"; expires="+u.expires.toUTCString():"",u.path?"; path="+u.path:"",u.domain?"; domain="+u.domain:"",u.secure?"; secure":""].join("")}for(var p=i?void 0:{},d=document.cookie?document.cookie.split("; "):[],v=0,x=d.length;v<x;v++){var k=d[v].split("="),l=r(k.shift()),j=k.join("=");if(i&&i===l){p=n(j,c);break}i||void 0===(j=n(j))||(p[l]=j)}return p};t.defaults={},e.removeCookie=function(i,o){return void 0!==e.cookie(i)&&(e.cookie(i,"",e.extend({},o,{expires:-1})),!e.cookie(i))}});
