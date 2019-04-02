module Common exposing (queryView)

import Application.Application as Application
import Html
import Message.TopLevelMessage exposing (TopLevelMessage)
import Test.Html.Query as Query


queryView : Application.Model -> Query.Single TopLevelMessage
queryView =
    Application.view
        >> .body
        >> List.head
        >> Maybe.withDefault (Html.text "")
        >> Query.fromHtml
