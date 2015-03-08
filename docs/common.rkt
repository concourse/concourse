#lang scheme/base

(require racket/cmdline
         scribble/core
         scribble/html-properties
         scriblib/render-cond)

(provide version)

(define version-param (make-parameter "x.x.x"))
(define analytics-id (make-parameter ""))
(define gauges-id (make-parameter ""))

(command-line
   #:once-each
   ["--version" v "Version of Concourse the docs are for" (version-param v)]
   ["--analytics-id" i "Google Analytics site ID" (analytics-id i)]
   ["--gauges-id" i "Gauges site ID" (gauges-id i)])

(define version (version-param))

(provide strike)

(define strike (make-style "strike"
  (list (make-css-addition "concourse.css"))))

(provide inject-analytics)

(define (inject-analytics)
  (cond-element
    [latex ""]
    [html (make-element
            (make-style #f
              (list
                (make-script-property
                  "text/javascript"
                  (append
                    (analytics-script (analytics-id))
                    (gauges-script (gauges-id))))))
            '())]
    [text ""]))

(define (analytics-script site-id)
  (if (equal? "" site-id)
    (list)
    (list
      "(function(i,s,o,g,r,a,m){i['GoogleAnalyticsObject']=r;i[r]=i[r]||function(){\n"
      "(i[r].q=i[r].q||[]).push(arguments)},i[r].l=1*new Date();a=s.createElement(o),\n"
      "m=s.getElementsByTagName(o)[0];a.async=1;a.src=g;m.parentNode.insertBefore(a,m)\n"
      "})(window,document,'script','//www.google-analytics.com/analytics.js','ga');\n"
      "\n"
      "ga('create', '" site-id "', 'auto');\n"
      "ga('send', 'pageview');\n")))

(define (gauges-script site-id)
  (if (equal? "" site-id)
    (list)
    (list
      "var _gauges = _gauges || [];\n"
      "(function() {\n"
      "  var t   = document.createElement('script');\n"
      "  t.type  = 'text/javascript';\n"
      "  t.async = true;\n"
      "  t.id    = 'gauges-tracker';\n"
      "  t.setAttribute('data-site-id', '" site-id "');\n"
      "  t.setAttribute('data-track-path', 'https://track.gaug.es/track.gif');\n"
      "  t.src = 'https://track.gaug.es/track.js';\n"
      "  var s = document.getElementsByTagName('script')[0];\n"
      "  s.parentNode.insertBefore(t, s);\n"
      "})();\n")))
