module Resource
    exposing
        ( Flags
        , Msg(..)
        , Model
        , init
        , changeToResource
        , update
        , updateWithMessage
        , view
        , viewPinButton
        , viewVersionHeader
        , viewVersionBody
        , subscriptions
        )

import Concourse
import Concourse.BuildStatus
import Concourse.Pagination exposing (Pagination, Paginated, Page, equal)
import Concourse.Resource
import Css
import Dict
import DictView
import Date exposing (Date)
import Date.Format
import Duration exposing (Duration)
import Erl
import Html.Styled as Html exposing (Html)
import Html.Styled.Attributes exposing (class, css, href, id, style, title)
import Html.Styled.Events exposing (onClick, onMouseEnter, onMouseLeave, onMouseOver, onMouseOut)
import Http
import List.Extra as LE
import Maybe.Extra as ME
import Navigation
import NewTopBar.Styles as Styles
import Pinned exposing (Pinned(..), PinState(..))
import StrictEvents
import Task exposing (Task)
import Time exposing (Time)
import LoginRedirect
import UpdateMsg exposing (UpdateMsg)


type alias Ports =
    { title : String -> Cmd Msg
    }


type PageError
    = Empty
    | NotFound


type alias Model =
    { ports : Ports
    , pageStatus : Result PageError ()
    , teamName : String
    , pipelineName : String
    , name : String
    , failingToCheck : Bool
    , checkError : String
    , checkSetupError : String
    , lastChecked : Maybe Date
    , pinnedVersion : Pinned Concourse.Version Int
    , now : Maybe Time.Time
    , resourceIdentifier : Concourse.ResourceIdentifier
    , versionedResources : Paginated Concourse.VersionedResource
    , currentPage : Maybe Page
    , versionedUIStates : Dict.Dict Int VersionUIState
    , csrfToken : String
    , showPinBarTooltip : Bool
    }


type alias VersionUIState =
    { expanded : Bool
    , inputTo : List Concourse.Build
    , outputOf : List Concourse.Build
    , showTooltip : Bool
    }


type Msg
    = Noop
    | AutoupdateTimerTicked Time
    | ResourceFetched (Result Http.Error Concourse.Resource)
    | VersionedResourcesFetched (Maybe Page) (Result Http.Error (Paginated Concourse.VersionedResource))
    | LoadPage Page
    | ClockTick Time.Time
    | ExpandVersionedResource Int
    | InputToFetched Int (Result Http.Error (List Concourse.Build))
    | OutputOfFetched Int (Result Http.Error (List Concourse.Build))
    | NavTo String
    | TogglePinBarTooltip
    | ToggleVersionTooltip
    | PinVersion Int
    | UnpinVersion
    | VersionPinned (Result Http.Error ())
    | VersionUnpinned (Result Http.Error ())


type alias Flags =
    { teamName : String
    , pipelineName : String
    , resourceName : String
    , paging : Maybe Concourse.Pagination.Page
    , csrfToken : String
    }


init : Ports -> Flags -> ( Model, Cmd Msg )
init ports flags =
    let
        ( model, cmd ) =
            changeToResource flags
                { resourceIdentifier =
                    { teamName = flags.teamName
                    , pipelineName = flags.pipelineName
                    , resourceName = flags.resourceName
                    }
                , pageStatus = Err Empty
                , teamName = flags.teamName
                , pipelineName = flags.pipelineName
                , name = flags.resourceName
                , failingToCheck = False
                , checkError = ""
                , checkSetupError = ""
                , lastChecked = Nothing
                , pinnedVersion = Off
                , currentPage = Nothing
                , versionedResources =
                    { content = []
                    , pagination =
                        { previousPage = Nothing
                        , nextPage = Nothing
                        }
                    }
                , versionedUIStates = Dict.empty
                , ports = ports
                , now = Nothing
                , csrfToken = flags.csrfToken
                , showPinBarTooltip = False
                }
    in
        ( model
        , Cmd.batch
            [ fetchResource model.resourceIdentifier
            , cmd
            ]
        )


changeToResource : Flags -> Model -> ( Model, Cmd Msg )
changeToResource flags model =
    ( { model
        | currentPage = flags.paging
        , versionedResources =
            { content = []
            , pagination =
                { previousPage = Nothing
                , nextPage = Nothing
                }
            }
      }
    , fetchVersionedResources model.resourceIdentifier flags.paging
    )


updateWithMessage : Msg -> Model -> ( Model, Cmd Msg, Maybe UpdateMsg )
updateWithMessage message model =
    let
        ( mdl, msg ) =
            update message model
    in
        if mdl.pageStatus == Err NotFound then
            ( mdl, msg, Just UpdateMsg.NotFound )
        else
            ( mdl, msg, Nothing )


updatePinnedVersion : Concourse.Resource -> Model -> Model
updatePinnedVersion resource model =
    case ( resource.pinnedVersion, resource.pinnedInConfig ) of
        ( Nothing, _ ) ->
            case model.pinnedVersion of
                TurningOn d ->
                    model

                _ ->
                    { model | pinnedVersion = Off }

        ( Just v, True ) ->
            { model | pinnedVersion = Static v }

        ( Just newVersion, False ) ->
            case model.pinnedVersion of
                TurningOff _ ->
                    { model | pinnedVersion = TurningOff newVersion }

                _ ->
                    { model | pinnedVersion = On newVersion }


hasPinnedVersion : Model -> Concourse.Version -> Bool
hasPinnedVersion model v =
    case model.pinnedVersion of
        Static pv ->
            v == pv

        On pv ->
            v == pv

        TurningOff pv ->
            v == pv

        _ ->
            False


update : Msg -> Model -> ( Model, Cmd Msg )
update action model =
    case action of
        Noop ->
            ( model, Cmd.none )

        AutoupdateTimerTicked timestamp ->
            ( model
            , Cmd.batch <|
                List.append
                    [ fetchResource model.resourceIdentifier
                    , fetchVersionedResources model.resourceIdentifier model.currentPage
                    ]
                <|
                    updateExpandedProperties model
            )

        ResourceFetched (Ok resource) ->
            ( { model
                | pageStatus = Ok ()
                , teamName = resource.teamName
                , pipelineName = resource.pipelineName
                , name = resource.name
                , failingToCheck = resource.failingToCheck
                , checkError = resource.checkError
                , checkSetupError = resource.checkSetupError
                , lastChecked = resource.lastChecked
              }
                |> updatePinnedVersion resource
            , model.ports.title <| resource.name ++ " - "
            )

        ResourceFetched (Err err) ->
            case Debug.log ("failed to fetch resource") (err) of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, LoginRedirect.requestLoginRedirect "" )
                    else if status.code == 404 then
                        ( { model | pageStatus = Err NotFound }, Cmd.none )
                    else
                        ( model, Cmd.none )

                _ ->
                    ( model, Cmd.none )

        VersionedResourcesFetched requestedPage (Ok paginated) ->
            let
                fetchedPage =
                    permalink paginated.content

                newModel =
                    \newPage ->
                        { model
                            | versionedResources = paginated
                            , currentPage = newPage
                        }

                chosenModelWith =
                    \requestedPageUnwrapped ->
                        case model.currentPage of
                            Nothing ->
                                newModel <| Just fetchedPage

                            Just page ->
                                if Concourse.Pagination.equal page requestedPageUnwrapped then
                                    newModel <| requestedPage
                                else
                                    model
            in
                case requestedPage of
                    Nothing ->
                        ( newModel (Just fetchedPage), Cmd.none )

                    Just requestedPageUnwrapped ->
                        ( chosenModelWith requestedPageUnwrapped
                        , Cmd.none
                        )

        VersionedResourcesFetched _ (Err err) ->
            flip always (Debug.log ("failed to fetch versioned resources") (err)) <|
                ( model, Cmd.none )

        LoadPage page ->
            ( { model
                | currentPage = Just page
              }
            , Cmd.batch
                [ fetchVersionedResources model.resourceIdentifier <| Just page
                , Navigation.newUrl <| paginationRoute model.resourceIdentifier page
                ]
            )

        ExpandVersionedResource versionID ->
            let
                versionedResourceIdentifier =
                    { teamName = model.resourceIdentifier.teamName
                    , pipelineName = model.resourceIdentifier.pipelineName
                    , resourceName = model.resourceIdentifier.resourceName
                    , versionID = versionID
                    }

                oldState =
                    getState versionID model.versionedUIStates

                newState =
                    { oldState
                        | expanded = not oldState.expanded
                    }
            in
                ( { model
                    | versionedUIStates = setState versionID newState model.versionedUIStates
                  }
                , if newState.expanded then
                    Cmd.batch
                        [ fetchInputTo versionedResourceIdentifier
                        , fetchOutputOf versionedResourceIdentifier
                        ]
                  else
                    Cmd.none
                )

        InputToFetched _ (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, LoginRedirect.requestLoginRedirect "" )
                    else
                        ( model, Cmd.none )

                _ ->
                    ( model, Cmd.none )

        InputToFetched versionID (Ok builds) ->
            let
                oldState =
                    getState versionID model.versionedUIStates

                newState =
                    { oldState
                        | inputTo = builds
                    }
            in
                ( { model
                    | versionedUIStates = setState versionID newState model.versionedUIStates
                  }
                , Cmd.none
                )

        OutputOfFetched _ (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, LoginRedirect.requestLoginRedirect "" )
                    else
                        ( model, Cmd.none )

                _ ->
                    ( model, Cmd.none )

        ClockTick now ->
            ( { model | now = Just now }, Cmd.none )

        OutputOfFetched versionID (Ok builds) ->
            let
                oldState =
                    getState versionID model.versionedUIStates

                newState =
                    { oldState
                        | outputOf = builds
                    }
            in
                ( { model
                    | versionedUIStates = setState versionID newState model.versionedUIStates
                  }
                , Cmd.none
                )

        NavTo url ->
            ( model, Navigation.newUrl url )

        TogglePinBarTooltip ->
            ( { model
                | showPinBarTooltip =
                    case model.pinnedVersion of
                        Static _ ->
                            not model.showPinBarTooltip

                        _ ->
                            False
              }
            , Cmd.none
            )

        ToggleVersionTooltip ->
            let
                pinnedVersionID : Maybe Int
                pinnedVersionID =
                    model.versionedResources.content
                        |> LE.find (.version >> hasPinnedVersion model)
                        |> Maybe.map .id

                newModel =
                    case ( model.pinnedVersion, pinnedVersionID ) of
                        ( Static _, Just id ) ->
                            let
                                oldState =
                                    getState id model.versionedUIStates

                                newState =
                                    { oldState
                                        | showTooltip = not oldState.showTooltip
                                    }
                            in
                                { model
                                    | versionedUIStates = setState id newState model.versionedUIStates
                                }

                        _ ->
                            model
            in
                ( newModel, Cmd.none )

        PinVersion versionID ->
            let
                versionedResource : Maybe Concourse.VersionedResource
                versionedResource =
                    model.versionedResources.content
                        |> LE.find (\vr -> vr.id == versionID)

                cmd : Cmd Msg
                cmd =
                    case versionedResource of
                        Just vr ->
                            Task.attempt VersionPinned <|
                                Concourse.Resource.pinVersion
                                    { teamName = model.resourceIdentifier.teamName
                                    , pipelineName = model.resourceIdentifier.pipelineName
                                    , resourceName = model.resourceIdentifier.resourceName
                                    , versionID = vr.id
                                    }
                                    model.csrfToken

                        Nothing ->
                            Cmd.none

                newModel =
                    { model | pinnedVersion = Pinned.startPinningTo versionID model.pinnedVersion }
            in
                ( newModel
                , cmd
                )

        UnpinVersion ->
            let
                pinnedVersionedResource : Maybe Concourse.VersionedResource
                pinnedVersionedResource =
                    model.versionedResources.content
                        |> LE.find (.version >> hasPinnedVersion model)

                cmd : Cmd Msg
                cmd =
                    case pinnedVersionedResource of
                        Just vr ->
                            Task.attempt VersionUnpinned <|
                                Concourse.Resource.unpinVersion
                                    { teamName = model.resourceIdentifier.teamName
                                    , pipelineName = model.resourceIdentifier.pipelineName
                                    , resourceName = model.resourceIdentifier.resourceName
                                    , versionID = vr.id
                                    }
                                    model.csrfToken

                        Nothing ->
                            Cmd.none
            in
                ( { model | pinnedVersion = Pinned.startUnpinning model.pinnedVersion }, cmd )

        VersionPinned (Ok ()) ->
            let
                newPinnedVersion =
                    Pinned.finishPinning
                        (\pinningTo ->
                            model.versionedResources.content
                                |> LE.find (\vr -> vr.id == pinningTo)
                                |> Maybe.map .version
                        )
                        model.pinnedVersion
            in
                ( { model | pinnedVersion = newPinnedVersion }, Cmd.none )

        VersionPinned (Err _) ->
            ( { model
                | pinnedVersion = Off
              }
            , Cmd.none
            )

        VersionUnpinned (Ok ()) ->
            ( { model
                | pinnedVersion = Off
              }
            , Cmd.none
            )

        VersionUnpinned (Err _) ->
            ( { model
                | pinnedVersion = Pinned.quitUnpinning model.pinnedVersion
              }
            , Cmd.none
            )


permalink : List Concourse.VersionedResource -> Page
permalink versionedResources =
    case List.head versionedResources of
        Nothing ->
            { direction = Concourse.Pagination.Since 0
            , limit = 100
            }

        Just version ->
            { direction = Concourse.Pagination.From version.id
            , limit = List.length versionedResources
            }


paginationRoute : Concourse.ResourceIdentifier -> Page -> String
paginationRoute rid page =
    let
        ( param, boundary ) =
            case page.direction of
                Concourse.Pagination.Since bound ->
                    ( "since", Basics.toString bound )

                Concourse.Pagination.Until bound ->
                    ( "until", Basics.toString bound )

                Concourse.Pagination.From bound ->
                    ( "from", Basics.toString bound )

                Concourse.Pagination.To bound ->
                    ( "to", Basics.toString bound )

        parsedRoute =
            Erl.parse <|
                "/teams/"
                    ++ rid.teamName
                    ++ "/pipelines/"
                    ++ rid.pipelineName
                    ++ "/resources/"
                    ++ rid.resourceName

        newParsedRoute =
            Erl.addQuery param boundary <| Erl.addQuery "limit" (Basics.toString page.limit) parsedRoute
    in
        Erl.toString newParsedRoute


view : Model -> Html Msg
view model =
    if model.pageStatus == Err Empty then
        Html.div [] []
    else
        let
            ( checkStatus, checkMessage, stepBody ) =
                if model.failingToCheck then
                    if not (String.isEmpty model.checkSetupError) then
                        ( "fr errored fa fa-fw fa-exclamation-triangle"
                        , "checking failed"
                        , [ Html.div [ class "step-body" ]
                                [ Html.pre [] [ Html.text model.checkSetupError ]
                                ]
                          ]
                        )
                    else
                        ( "fr errored fa fa-fw fa-exclamation-triangle"
                        , "checking failed"
                        , [ Html.div [ class "step-body" ]
                                [ Html.pre [] [ Html.text model.checkError ]
                                ]
                          ]
                        )
                else
                    ( "fr succeeded fa fa-fw fa-check", "checking successfully", [] )

            ( previousButtonClass, previousButtonEvent ) =
                case model.versionedResources.pagination.previousPage of
                    Nothing ->
                        ( "btn-page-link prev disabled", Noop )

                    Just pp ->
                        ( "btn-page-link prev", LoadPage pp )

            ( nextButtonClass, nextButtonEvent ) =
                case model.versionedResources.pagination.nextPage of
                    Nothing ->
                        ( "btn-page-link next disabled", Noop )

                    Just np ->
                        let
                            updatedPage =
                                { np
                                    | limit = 100
                                }
                        in
                            ( "btn-page-link next", LoadPage updatedPage )

            lastCheckedView =
                case ( model.now, model.lastChecked ) of
                    ( Just now, Just date ) ->
                        viewLastChecked now date

                    ( _, _ ) ->
                        Html.text ""

            pinBarVersion =
                Pinned.stable model.pinnedVersion

            pinBar =
                Html.div
                    ([ css
                        [ Css.flexGrow (Css.num 1)
                        , Css.margin (Css.px 10)
                        , Css.paddingLeft (Css.px 7)
                        , Css.displayFlex
                        , Css.alignItems Css.center
                        , Css.position Css.relative
                        ]
                     , style
                        [ ( "border"
                          , "1px solid "
                                ++ (if ME.isJust pinBarVersion then
                                        "#03dac4"
                                    else
                                        "#3d3c3c"
                                   )
                          )
                        ]
                     , id "pin-bar"
                     ]
                        ++ (case model.pinnedVersion of
                                Static _ ->
                                    [ onMouseEnter TogglePinBarTooltip
                                    , onMouseLeave TogglePinBarTooltip
                                    ]

                                _ ->
                                    []
                           )
                    )
                    ([ Html.div
                        [ css
                            [ Css.backgroundRepeat Css.noRepeat
                            , Css.backgroundPosition2 (Css.pct 50) (Css.pct 50)
                            , Css.height (Css.px 15)
                            , Css.width (Css.px 15)
                            , Css.marginRight (Css.px 10)
                            ]
                        , style
                            [ ( "background-image"
                              , if ME.isJust pinBarVersion then
                                    "url(/public/images/pin_ic_white.svg)"
                                else
                                    "url(/public/images/pin_ic_grey.svg)"
                              )
                            ]
                        ]
                        []
                     ]
                        ++ (case pinBarVersion of
                                Just v ->
                                    [ viewVersion v ]

                                _ ->
                                    []
                           )
                        ++ (if model.showPinBarTooltip then
                                [ Html.div
                                    [ css
                                        [ Css.position Css.absolute
                                        , Css.top <| Css.px -10
                                        , Css.left <| Css.px 30
                                        , Css.backgroundColor <| Css.hex "9b9b9b"
                                        , Css.zIndex <| Css.int 2
                                        , Css.padding <| Css.px 5
                                        ]
                                    ]
                                    [ Html.text "pinned in pipeline config" ]
                                ]
                            else
                                []
                           )
                    )

            headerHeight =
                60
        in
            Html.div []
                [ Html.div
                    [ css
                        [ Css.height <| Css.px headerHeight
                        , Css.position Css.fixed
                        , Css.top <| Css.px Styles.pageHeaderHeight
                        , Css.displayFlex
                        , Css.alignItems Css.stretch
                        , Css.width <| Css.pct 100
                        , Css.zIndex <| Css.int 1
                        , Css.backgroundColor <| Css.hex "2a2929"
                        ]
                    ]
                    [ Html.h1
                        [ css
                            [ Css.fontWeight <| Css.int 700
                            , Css.marginLeft <| Css.px 18
                            , Css.displayFlex
                            , Css.alignItems Css.center
                            , Css.justifyContent Css.center
                            ]
                        ]
                        [ Html.text model.name ]
                    , Html.div
                        [ css
                            [ Css.displayFlex
                            , Css.alignItems Css.center
                            , Css.justifyContent Css.center
                            , Css.marginLeft (Css.px 24)
                            ]
                        ]
                        [ lastCheckedView ]
                    , pinBar
                    , Html.div
                        [ class previousButtonClass
                        , onClick previousButtonEvent
                        , css [ Css.displayFlex, Css.alignItems Css.center ]
                        ]
                        [ Html.a [ class "arrow" ]
                            [ Html.i [ class "fa fa-arrow-left" ] []
                            ]
                        ]
                    , Html.div
                        [ class nextButtonClass
                        , onClick nextButtonEvent
                        , css [ Css.displayFlex, Css.alignItems Css.center ]
                        ]
                        [ Html.a [ class "arrow" ]
                            [ Html.i [ class "fa fa-arrow-right" ] []
                            ]
                        ]
                    ]
                , Html.div
                    [ css
                        [ Css.padding3 (Css.px <| headerHeight + 10) (Css.px 10) (Css.px 10)
                        ]
                    ]
                    [ Html.div [ class "resource-check-status" ]
                        [ Html.div [ class "build-step" ]
                            (List.append
                                [ Html.div [ class "header" ]
                                    [ Html.h3 [] [ Html.text checkMessage ]
                                    , Html.i [ class <| checkStatus ] []
                                    ]
                                ]
                                stepBody
                            )
                        ]
                    , viewVersionedResources model
                    ]
                ]


checkForVersionID : Int -> Concourse.VersionedResource -> Bool
checkForVersionID versionID versionedResource =
    versionID == versionedResource.id


viewVersionedResources :
    { a
        | versionedResources : Paginated Concourse.VersionedResource
        , versionedUIStates : Dict.Dict Int VersionUIState
        , pinnedVersion : Pinned Concourse.Version Int
    }
    -> Html Msg
viewVersionedResources { versionedResources, versionedUIStates, pinnedVersion } =
    versionedResources.content
        |> List.map
            (\vr ->
                viewVersionedResource
                    { versionedResource = vr
                    , pinState = Pinned.pinState vr.version vr.id pinnedVersion
                    , state =
                        let
                            state =
                                getState vr.id versionedUIStates

                            showTooltip =
                                case pinnedVersion of
                                    Static _ ->
                                        state.showTooltip

                                    _ ->
                                        False
                        in
                            { state | showTooltip = showTooltip }
                    , isDynamic =
                        case pinnedVersion of
                            Static _ ->
                                False

                            _ ->
                                True
                    }
            )
        |> Html.ul [ class "list list-collapsable list-enableDisable resource-versions" ]


viewVersionedResource :
    { versionedResource : Concourse.VersionedResource
    , state : VersionUIState
    , pinState : PinState
    , isDynamic : Bool
    }
    -> Html Msg
viewVersionedResource { versionedResource, pinState, state, isDynamic } =
    Html.li
        (case pinState of
            Disabled ->
                [ style [ ( "opacity", "0.5" ) ] ]

            _ ->
                []
        )
        ([ Html.div
            [ css
                [ Css.displayFlex
                , Css.margin2 (Css.px 5) Css.zero
                ]
            ]
            [ viewPinButton
                { versionID = versionedResource.id
                , pinState = pinState
                , showTooltip = state.showTooltip
                , isDynamic = isDynamic
                }
            , viewVersionHeader
                { id = versionedResource.id
                , version = versionedResource.version
                , pinnedState = pinState
                }
            ]
         ]
            ++ (if state.expanded then
                    [ viewVersionBody
                        { inputTo = state.inputTo
                        , outputOf = state.outputOf
                        , metadata = versionedResource.metadata
                        }
                    ]
                else
                    []
               )
        )


viewVersionBody :
    { a
        | inputTo : List Concourse.Build
        , outputOf : List Concourse.Build
        , metadata : Concourse.Metadata
    }
    -> Html Msg
viewVersionBody { inputTo, outputOf, metadata } =
    Html.div
        [ css
            [ Css.displayFlex
            , Css.padding2 (Css.px 5) (Css.px 10)
            ]
        ]
        [ Html.div [ class "vri" ] <|
            List.concat
                [ [ Html.div [ css [ Css.lineHeight <| Css.px 25 ] ] [ Html.text "inputs to" ] ]
                , viewBuilds <| listToMap inputTo
                ]
        , Html.div [ class "vri" ] <|
            List.concat
                [ [ Html.div [ css [ Css.lineHeight <| Css.px 25 ] ] [ Html.text "outputs of" ] ]
                , viewBuilds <| listToMap outputOf
                ]
        , Html.div [ class "vri metadata-container" ]
            [ Html.div [ class "list-collapsable-title" ] [ Html.text "metadata" ]
            , viewMetadata metadata
            ]
        ]


viewPinButton : { versionID : Int, pinState : PinState, showTooltip : Bool, isDynamic : Bool } -> Html Msg
viewPinButton { versionID, pinState, showTooltip, isDynamic } =
    Html.div
        ([ Html.Styled.Attributes.attribute "aria-label" "Pin Resource Version"
         , css
            [ Css.position Css.relative
            , Css.backgroundRepeat Css.noRepeat
            , Css.backgroundPosition2 (Css.pct 50) (Css.pct 50)
            , Css.marginRight (Css.px 5)
            , Css.width (Css.px 25)
            , Css.height (Css.px 25)
            , Css.float Css.left
            , Css.cursor Css.default
            ]
         , style
            ([ ( "background-color", "#1e1d1d" ) ]
                ++ (if pinState /= Pending then
                        [ ( "background-image", "url(/public/images/pin_ic_white.svg)" ) ]
                    else
                        []
                   )
                ++ (if pinState == Pinned then
                        [ ( "border", "1px solid #03dac4" ) ]
                    else
                        []
                   )
            )
         ]
            ++ (case ( pinState, isDynamic ) of
                    ( Enabled, True ) ->
                        [ onClick <| PinVersion versionID ]

                    ( Pinned, True ) ->
                        [ onClick <| UnpinVersion ]

                    _ ->
                        []
               )
            ++ (if pinState == Pinned then
                    [ onMouseOut ToggleVersionTooltip
                    , onMouseOver ToggleVersionTooltip
                    ]
                else
                    []
               )
        )
        ((if showTooltip then
            [ Html.div
                [ css
                    [ Css.position Css.absolute
                    , Css.bottom <| Css.px 25
                    , Css.backgroundColor <| Css.hex "9b9b9b"
                    , Css.zIndex <| Css.int 2
                    , Css.padding <| Css.px 5
                    , Css.width <| Css.px 170
                    ]
                ]
                [ Html.text "enable via pipeline config" ]
            ]
          else
            []
         )
            ++ (if pinState == Pending then
                    [ Html.text "..." ]
                else
                    []
               )
        )


viewVersionHeader : { a | id : Int, version : Concourse.Version, pinnedState : PinState } -> Html Msg
viewVersionHeader { id, version, pinnedState } =
    Html.div
        ([ css
            [ Css.flexGrow <| Css.num 1
            , Css.backgroundColor <| Css.hex "1e1d1d"
            , Css.cursor Css.pointer
            , Css.displayFlex
            , Css.alignItems Css.center
            , Css.paddingLeft <| Css.px 10
            , Css.color <| Css.hex <| "e6e7e8"
            ]
         , onClick <| ExpandVersionedResource id
         ]
            ++ (case pinnedState of
                    Pinned ->
                        [ style [ ( "border", "1px solid #03dac4" ) ] ]

                    _ ->
                        []
               )
        )
        [ viewVersion version ]


getState : Int -> Dict.Dict Int VersionUIState -> VersionUIState
getState versionID states =
    let
        resourceState =
            Dict.get versionID states
    in
        case resourceState of
            Nothing ->
                { expanded = False
                , inputTo = []
                , outputOf = []
                , showTooltip = False
                }

            Just rs ->
                rs


setState : Int -> VersionUIState -> Dict.Dict Int VersionUIState -> Dict.Dict Int VersionUIState
setState versionID newState states =
    Dict.insert versionID newState states


viewVersion : Concourse.Version -> Html Msg
viewVersion version =
    (Html.fromUnstyled << DictView.view)
        << Dict.map (\_ s -> Html.toUnstyled (Html.text s))
    <|
        version


viewMetadata : Concourse.Metadata -> Html Msg
viewMetadata metadata =
    Html.dl [ class "build-metadata" ]
        (List.concatMap viewMetadataField metadata)


viewMetadataField : Concourse.MetadataField -> List (Html a)
viewMetadataField field =
    [ Html.dt [] [ Html.text field.name ]
    , Html.dd []
        [ Html.pre [ class "metadata-field" ] [ Html.text field.value ]
        ]
    ]


listToMap : List Concourse.Build -> Dict.Dict String (List Concourse.Build)
listToMap builds =
    let
        insertBuild =
            \build dict ->
                let
                    jobName =
                        case build.job of
                            Nothing ->
                                Debug.crash "Jobless builds shouldn't appear on this page!" ""

                            Just job ->
                                job.jobName

                    oldList =
                        Dict.get jobName dict

                    newList =
                        case oldList of
                            Nothing ->
                                [ build ]

                            Just list ->
                                list ++ [ build ]
                in
                    Dict.insert jobName newList dict
    in
        List.foldr insertBuild Dict.empty builds


viewBuilds : Dict.Dict String (List Concourse.Build) -> List (Html Msg)
viewBuilds buildDict =
    List.concatMap (viewBuildsByJob buildDict) <| Dict.keys buildDict


viewLastChecked : Time -> Date -> Html a
viewLastChecked now date =
    let
        ago =
            Duration.between (Date.toTime date) now
    in
        Html.table []
            [ Html.tr
                []
                [ Html.td [] [ Html.text "checked" ]
                , Html.td [ title (Date.Format.format "%b %d %Y %I:%M:%S %p" date) ]
                    [ Html.span [] [ Html.text (Duration.format ago ++ " ago") ] ]
                ]
            ]


viewBuildsByJob : Dict.Dict String (List Concourse.Build) -> String -> List (Html Msg)
viewBuildsByJob buildDict jobName =
    let
        oneBuildToLi =
            \build ->
                let
                    link =
                        case build.job of
                            Nothing ->
                                ""

                            Just job ->
                                "/teams/" ++ job.teamName ++ "/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.jobName ++ "/builds/" ++ build.name
                in
                    Html.li [ class <| Concourse.BuildStatus.show build.status ]
                        [ Html.a
                            [ Html.Styled.Attributes.fromUnstyled <| StrictEvents.onLeftClick <| NavTo link
                            , href link
                            ]
                            [ Html.text <| "#" ++ build.name ]
                        ]
    in
        [ Html.h3 [ class "man pas ansi-bright-black-bg" ] [ Html.text jobName ]
        , Html.ul [ class "builds-list" ]
            (case (Dict.get jobName buildDict) of
                Nothing ->
                    []

                -- never happens
                Just buildList ->
                    (List.map oneBuildToLi buildList)
            )
        ]


updateExpandedProperties : Model -> List (Cmd Msg)
updateExpandedProperties model =
    let
        filteredList =
            List.filter
                (isExpanded model.versionedUIStates)
                model.versionedResources.content
    in
        List.concatMap
            (fetchInputAndOutputs model)
            filteredList


isExpanded : Dict.Dict Int VersionUIState -> Concourse.VersionedResource -> Bool
isExpanded states versionedResource =
    let
        state =
            Dict.get versionedResource.id states
    in
        case state of
            Nothing ->
                False

            Just someState ->
                someState.expanded


fetchInputAndOutputs : Model -> Concourse.VersionedResource -> List (Cmd Msg)
fetchInputAndOutputs model versionedResource =
    let
        identifier =
            { teamName = model.resourceIdentifier.teamName
            , pipelineName = model.resourceIdentifier.pipelineName
            , resourceName = model.resourceIdentifier.resourceName
            , versionID = versionedResource.id
            }
    in
        [ fetchInputTo identifier
        , fetchOutputOf identifier
        ]


fetchResource : Concourse.ResourceIdentifier -> Cmd Msg
fetchResource resourceIdentifier =
    Task.attempt ResourceFetched <|
        Concourse.Resource.fetchResource resourceIdentifier


fetchVersionedResources : Concourse.ResourceIdentifier -> Maybe Page -> Cmd Msg
fetchVersionedResources resourceIdentifier page =
    Task.attempt (VersionedResourcesFetched page) <|
        Concourse.Resource.fetchVersionedResources resourceIdentifier page


fetchInputTo : Concourse.VersionedResourceIdentifier -> Cmd Msg
fetchInputTo versionedResourceIdentifier =
    Task.attempt (InputToFetched versionedResourceIdentifier.versionID) <|
        Concourse.Resource.fetchInputTo versionedResourceIdentifier


fetchOutputOf : Concourse.VersionedResourceIdentifier -> Cmd Msg
fetchOutputOf versionedResourceIdentifier =
    Task.attempt (OutputOfFetched versionedResourceIdentifier.versionID) <|
        Concourse.Resource.fetchOutputOf versionedResourceIdentifier


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.batch
        [ Time.every (5 * Time.second) AutoupdateTimerTicked
        , Time.every Time.second ClockTick
        ]
