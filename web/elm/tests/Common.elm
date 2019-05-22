module Common exposing
    ( defineHoverBehaviour
    , init
    , isColorWithStripes
    , pipelineRunningKeyframes
    , queryView
    )

import Application.Application as Application
import Expect exposing (Expectation)
import Html
import Message.Effects exposing (Effect)
import Message.Message exposing (DomID, Message(..))
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (Selector, style)
import Url


queryView : Application.Model -> Query.Single TopLevelMessage
queryView =
    Application.view
        >> .body
        >> List.head
        >> Maybe.withDefault (Html.text "")
        >> Query.fromHtml


isColorWithStripes :
    { thick : String, thin : String }
    -> Query.Single msg
    -> Expectation
isColorWithStripes { thick, thin } =
    Query.has
        [ style "background-image" <|
            "repeating-linear-gradient(-115deg,"
                ++ thick
                ++ " 0,"
                ++ thick
                ++ " 10px,"
                ++ thin
                ++ " 0,"
                ++ thin
                ++ " 16px)"
        , style "background-size" "106px 114px"
        , style "animation" <|
            pipelineRunningKeyframes
                ++ " 3s linear infinite"
        ]


pipelineRunningKeyframes : String
pipelineRunningKeyframes =
    "pipeline-running"


defineHoverBehaviour :
    { name : String
    , setup : Application.Model
    , query : Application.Model -> Query.Single TopLevelMessage
    , unhoveredSelector : { description : String, selector : List Selector }
    , hoverable : DomID
    , hoveredSelector : { description : String, selector : List Selector }
    }
    -> Test
defineHoverBehaviour { name, setup, query, unhoveredSelector, hoverable, hoveredSelector } =
    describe (name ++ " hover behaviour")
        [ test (name ++ " is " ++ unhoveredSelector.description) <|
            \_ ->
                setup
                    |> query
                    |> Query.has unhoveredSelector.selector
        , test ("mousing over " ++ name ++ " triggers Hover msg") <|
            \_ ->
                setup
                    |> query
                    |> Event.simulate Event.mouseEnter
                    |> Event.expect (Update <| Hover <| Just hoverable)
        , test
            ("Hover msg causes "
                ++ name
                ++ " to become "
                ++ hoveredSelector.description
            )
          <|
            \_ ->
                setup
                    |> Application.update (Update <| Hover <| Just hoverable)
                    |> Tuple.first
                    |> query
                    |> Query.has hoveredSelector.selector
        , test ("mousing off " ++ name ++ " triggers unhover msg") <|
            \_ ->
                setup
                    |> Application.update (Update <| Hover <| Just hoverable)
                    |> Tuple.first
                    |> query
                    |> Event.simulate Event.mouseLeave
                    |> Event.expect (Update <| Hover Nothing)
        , test
            ("unhover msg causes "
                ++ name
                ++ " to become "
                ++ unhoveredSelector.description
            )
          <|
            \_ ->
                setup
                    |> Application.update (Update <| Hover <| Just hoverable)
                    |> Tuple.first
                    |> Application.update (Update <| Hover Nothing)
                    |> Tuple.first
                    |> query
                    |> Query.has unhoveredSelector.selector
        ]


init : String -> Application.Model
init path =
    Application.init
        { turbulenceImgSrc = ""
        , notFoundImgSrc = "notfound.svg"
        , csrfToken = "csrf_token"
        , authToken = ""
        , clusterName = ""
        , pipelineRunningKeyframes = "pipeline-running"
        }
        { protocol = Url.Http
        , host = ""
        , port_ = Nothing
        , path = path
        , query = Nothing
        , fragment = Nothing
        }
        |> Tuple.first



-- 6 places where Application.init is used with a query
-- 6 places where Application.init is used with a fragment
-- 1 place where Application.init is used with an instance name
