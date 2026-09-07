module PipelineInfo.PipelineInfo exposing
    ( Model
    , changeToPipeline
    , documentTitle
    , getUpdateMessage
    , handleCallback
    , handleDelivery
    , init
    , subscriptions
    , tooltip
    , update
    , view
    )

import Application.Models exposing (Session)
import Concourse
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes exposing (class, id)
import Http
import Json.Decode
import Json.Encode
import Login.Login as Login
import Markdown.Parser
import Markdown.Renderer
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (Message(..))
import Message.Subscription exposing (Delivery(..), Interval(..), Subscription(..))
import PipelineInfo.Styles as Styles
import RemoteData exposing (WebData)
import Routes
import SideBar.SideBar as SideBar
import Tooltip
import UpdateMsg exposing (UpdateMsg)
import Views.Styles
import Views.TopBar as TopBar
import Yaml


type alias Model =
    Login.Model
        { pipelineLocator : Concourse.PipelineIdentifier
        , pipeline : WebData Concourse.Pipeline
        }


init : Concourse.PipelineIdentifier -> ( Model, List Effect )
init pipelineLocator =
    ( { pipelineLocator = pipelineLocator
      , pipeline = RemoteData.NotAsked
      , isUserMenuExpanded = False
      }
    , [ FetchPipeline pipelineLocator
      , FetchAllPipelines
      ]
    )


changeToPipeline : Concourse.PipelineIdentifier -> ET Model
changeToPipeline pipelineLocator ( model, effects ) =
    if model.pipelineLocator == pipelineLocator then
        ( model, effects )

    else
        let
            ( newModel, newEffects ) =
                init pipelineLocator
        in
        ( newModel, effects ++ newEffects )


documentTitle : Model -> String
documentTitle model =
    model.pipelineLocator.pipelineName


getUpdateMessage : Model -> UpdateMsg
getUpdateMessage model =
    case model.pipeline of
        RemoteData.Failure _ ->
            UpdateMsg.NotFound

        _ ->
            UpdateMsg.AOK


handleCallback : Callback -> ET Model
handleCallback callback ( model, effects ) =
    case callback of
        PipelineFetched (Ok pipeline) ->
            ( { model
                | pipeline = RemoteData.Success pipeline
                , pipelineLocator = Concourse.toPipelineId pipeline
              }
            , effects
            )

        PipelineFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 404 then
                        ( { model | pipeline = RemoteData.Failure err }, effects )

                    else if status.code == 401 then
                        ( model, effects ++ [ RedirectToLogin ] )

                    else
                        ( model, effects )

                _ ->
                    ( model, effects )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET Model
handleDelivery delivery ( model, effects ) =
    case delivery of
        ClockTicked FiveSeconds _ ->
            ( model
            , effects
                ++ [ FetchPipeline model.pipelineLocator
                   , FetchAllPipelines
                   ]
            )

        _ ->
            ( model, effects )


update : Message -> ET Model
update _ ( model, effects ) =
    ( model, effects )


subscriptions : List Subscription
subscriptions =
    [ OnClockTick FiveSeconds ]


tooltip : Model -> Session -> Maybe Tooltip.Tooltip
tooltip _ _ =
    Nothing


view : Session -> Model -> Html Message
view session model =
    let
        route =
            Routes.PipelineInfo model.pipelineLocator
    in
    Html.div
        (id "page-including-top-bar" :: Views.Styles.pageIncludingTopBar)
        [ Views.Styles.hideIf session.hideUI
            (Html.div
                (id "top-bar-app" :: Views.Styles.topBar False)
                (SideBar.sideBarIcon session
                    :: TopBar.breadcrumbs session route
                    ++ [ Login.view session.userState model ]
                )
            )
        , Html.div
            (id "page-below-top-bar" :: Views.Styles.pageBelowTopBar session.hideUI route)
            [ Views.Styles.hideIf session.hideUI
                (SideBar.view session (Just model.pipelineLocator))
            , viewContent model
            ]
        ]


viewContent : Model -> Html Message
viewContent model =
    Html.div
        (id "pipeline-info" :: class "pipeline-info" :: Styles.page)
        [ Html.div (id "pipeline-info-body" :: Styles.body) <|
            case model.pipeline of
                RemoteData.Success pipeline ->
                    case pipeline.userData of
                        Just userData ->
                            viewUserData userData

                        Nothing ->
                            [ viewEmpty ]

                _ ->
                    []
        ]


viewEmpty : Html Message
viewEmpty =
    Html.div
        (id "pipeline-info-empty" :: Styles.empty)
        [ Html.text "this pipeline has no "
        , Html.code [] [ Html.text "user_data" ]
        , Html.text " configured"
        ]


{-| `user_data` is an opaque, arbitrary value, so pick a rendering from its
shape:

  - a bare string is treated as markdown;
  - an object with a `description` string renders that as markdown, with any
    remaining keys as YAML underneath;
  - anything else renders wholesale as YAML.

-}
viewUserData : Json.Decode.Value -> List (Html Message)
viewUserData userData =
    case Json.Decode.decodeValue Json.Decode.string userData of
        Ok markdown ->
            [ viewMarkdown markdown ]

        Err _ ->
            case splitDescription userData of
                Just ( markdown, [] ) ->
                    [ viewMarkdown markdown ]

                Just ( markdown, rest ) ->
                    [ viewMarkdown markdown
                    , viewYaml (Json.Encode.object rest)
                    ]

                Nothing ->
                    [ viewYaml userData ]


{-| Pulls a string `description` out of an object, returning it alongside the
object's other entries. `Nothing` when the value isn't an object or has no
string `description`.
-}
splitDescription : Json.Decode.Value -> Maybe ( String, List ( String, Json.Decode.Value ) )
splitDescription userData =
    case Json.Decode.decodeValue (Json.Decode.keyValuePairs Json.Decode.value) userData of
        Err _ ->
            Nothing

        Ok kvs ->
            let
                description =
                    kvs
                        |> List.filter (\( k, _ ) -> k == "description")
                        |> List.head
                        |> Maybe.map Tuple.second
                        |> Maybe.andThen
                            (Json.Decode.decodeValue Json.Decode.string >> Result.toMaybe)
            in
            description
                |> Maybe.map
                    (\d -> ( d, List.filter (\( k, _ ) -> k /= "description") kvs ))


viewYaml : Json.Decode.Value -> Html Message
viewYaml value =
    Html.pre
        (id "pipeline-info-yaml" :: Styles.yaml)
        [ Html.text (Yaml.fromJson value) ]


viewMarkdown : String -> Html Message
viewMarkdown markdown =
    Html.div
        (id "pipeline-info-markdown" :: class "markdown-body" :: Styles.markdown)
        (markdown
            |> Markdown.Parser.parse
            |> Result.mapError (always "")
            |> Result.andThen
                (Markdown.Renderer.render Markdown.Renderer.defaultHtmlRenderer)
            |> Result.withDefault [ Html.text markdown ]
        )
