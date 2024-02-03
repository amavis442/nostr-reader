import lightBolt11Decoder from 'light-bolt11-decoder';
import linkifyHtml from 'linkify-html';

//var urlRegex =/(\b(https?|ftp|file):\/\/[-A-Z0-9+&@#\/%?=~_|!:,.;]*[-A-Z0-9+&@#\/%=~_|])/ig;
const urlRegex = /https?:\/\/([\w.-]+)[^ ]*[-A-Z0-9+&@#/%=~_|]/ig;
//var imgUrlRegex = /https?:\/\/(?:[a-z\-]+\.)+[a-z]{2,6}(?:\/[^\/#?]+)+\.(?:jpe?g|gif|png|webp)/gmi;
const imgUrlRegex = /https?:\/\/.*\.(png|jpe?g|png|gif|webp)/igm;
const youtubeRegex = /(?:https?:\/\/)?(?:www\.)?(?:youtu\.be\/|youtube\.com\/(?:embed\/|v\/|watch\?v=|watch\?.+&v=))((\w|-){11})(?:\S+)?/gmi;
const rumbleRegex = /(?:https?:\/\/)?(?:www\.)?(?:rumble\.com\/([^-]+))-/gmi


export function findLink(text: string): Array<string> {
  let links: Array<string> = [];
  //text = text.replace(/\n/gm, " ")
  
  const m = ytVidId(text);
  if (m) links = [...links,...m];

  const r = rumbleVidId(text);
  if (m) links = [...links,...r];

  const p = imgTag(text);
  if (p) {
    //console.debug("Found img url matches in:\n [", text, "]\nResult: ", p)
    //Check if there a spaces in the output then seperate them
    let imgArray: Array<string> = [];
    for (let i = 0; i < p.length;i++) {
      if (p[i].match(" ")) {
        imgArray = [...imgArray, ...p[i].split(" ")]
      } else {
        imgArray = [...imgArray, ...p]
      }
    }
    links = [...links,...imgArray];
  }
  return links;
}

/**
 * JavaScript function to match (and return) the video Id 
 * of any valid Youtube Url, given as input string.
 * @author: Stephan Schmitz <eyecatchup@gmail.com>
 * @url: https://stackoverflow.com/a/10315969/624466
 */
function ytVidId(text: string): Array<string>  {
  const match = text.match(youtubeRegex);
  return (match && match[0]) ? [match[0]] : [];
}

function rumbleVidId(text: string): Array<string>  {
  const match = text.match(rumbleRegex);
  return (match && match[0]) ? [match[0]] : [];
}

function imgTag(text: string): Array<string> {
  //url = url.replace("<br />", ' ').replace("<br>", ' ')
  const match = text.match(imgUrlRegex);
  if (match && match.length > 0) {
    const imgUrls = []
    for (let i=0; i < match.length; i++) {
      imgUrls[i] = match[i]
    }
    return imgUrls;
  }
  return (match && match[0]) ? [match[0]] : [];
}



export function escapeHtml(html: string): string {
  const div = document.createElement("div");
  div.innerText = html;

  return div.innerHTML;
}

export function toHtml(content: string): string {
  const match = content.match(/(lnbc|lnbt)\w+/gmi);
  if (match && match[0]) { // Lightning invoice
    const invoice = lightBolt11Decoder.decode(match[0]);
    let amount = 0;
    let rawAmount = 0;
    let rawUnit = '';
    let unitNumber = 0;
    for (let i = 0; i < invoice.sections.length; i++) {
      if (invoice.sections[i]?.name == 'amount') {
        rawAmount = invoice.sections[i].value;
        rawUnit = invoice.sections[i].letters;
        amount = invoice.sections[i].value;
        const unit = invoice.sections[i].letters.replace(/[0-9]+/, '');
        unitNumber = invoice.sections[i].letters.replace(/[a-zA-Z]+/, '');
        switch (unit) {
          case 'm': amount = amount * 0.001 * unitNumber;
            break;
          case 'u': amount = amount * 0.000001 * unitNumber;
            break;
          case 'n': amount = amount * 0.000000001 * unitNumber;
            break;
          case 'p': amount = amount * 0.000000000001 * unitNumber;
            break;
        }
      }
    }
    //console.debug('INVOICE:', match, match[0], invoice)
    content = content.replace(match[0], 'lightning invoice: ' + amount + ' sats (Amount: ' + rawAmount + ', Unit: ' + rawUnit + ', Unit number: ' + unitNumber + ')');
  }
  content = content.replace("&#39;", "'");
  content = content.replace(/"/gm, "&#34;");
  content = content.replace(/\n/gm, "<br/>");
  content = content.replace('[~','<div class="mention">');
  content = content.replace('~]','</div>');
  
  
  const options = {
    defaultProtocol: 'https',
    attributes: {
      title: "External Link",
      class: "underline"
    },
    format: (value: string, type: string) => {
      if (type === "url") {
        value = value.replace(urlRegex, (url, domain) => {
            return `${domain}`;
        });
        if (value.length > 50) {
          value = value.slice(0, 50) + "…";
        }
      }
      return value;
    },
    rel: "noopener",
    target: {
      url: "_blank",
      email: null,
    },
  };
  content = linkifyHtml(content, options);
  //console.log(":After linkify", content)


  /* content = escapeHtml(content)
    .replace(/\n/g, "<br/>")
    //.replace(urlRegex, function(url, domain) {
    //  return `<a href="${url}"  target="_blank noopener" class="underline">${domain}</a>`;
    //});
    //.replace(urlRegex, (url, domain) => {
    //  return `<a href="${url}" target="_blank noopener" class="underline">${domain}</a>`;
    //})
  
    ;
*/
  //console.log(":Content is [", content, "]")
  return `<div>${content}</div>`;
}
