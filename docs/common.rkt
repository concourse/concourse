#lang scheme/base

(require scribble/core
         scribble/html-properties
         scriblib/render-cond)

(provide version)

(define version
  (let ([version-arg-provided (> (vector-length (current-command-line-arguments)) 0)])
    (if version-arg-provided
      (vector-ref (current-command-line-arguments) 0)
      "x.x.x")))

(provide strike)

(define strike (make-style "strike"
  (list (make-css-addition "concourse.css"))))

(provide inject-analytics)

(define (inject-analytics)
  (let ([analytics-token-provided (> (vector-length (current-command-line-arguments)) 1)])
    (if analytics-token-provided
      (cond-element
        [latex ""]
        [html (make-element
                (make-style #f
                    (list (make-script-property
                            "text/javascript"
                            (analytics-script (vector-ref (current-command-line-arguments) 1)))))
                '())]
        [text ""])
      "")))

(define (analytics-script token)
  (list
    "(function(i,s,o,g,r,a,m){i['GoogleAnalyticsObject']=r;i[r]=i[r]||function(){\n"
    "(i[r].q=i[r].q||[]).push(arguments)},i[r].l=1*new Date();a=s.createElement(o),\n"
    "m=s.getElementsByTagName(o)[0];a.async=1;a.src=g;m.parentNode.insertBefore(a,m)\n"
    "})(window,document,'script','//www.google-analytics.com/analytics.js','ga');\n"
    "\n"
    "ga('create', '" token "', 'auto');\n"
    "ga('send', 'pageview');\n"))
