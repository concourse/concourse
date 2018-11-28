module Resource
    exposing
        ( Flags
        , Msg(..)
        , Model
        , VersionToggleAction(..)
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

import BoolTransitionable
import Concourse
import Concourse.BuildStatus
import Concourse.Pagination exposing (Pagination, Paginated, Page, equal)
import Concourse.Resource
import Colors
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
import List.Extra
import Maybe.Extra as ME
import Navigation
import NewTopBar.Styles as Styles
import Pinned exposing (ResourcePinState(..), VersionPinState(..))
import Resource.Styles
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


type VersionToggleAction
    = Enable
    | Disable


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
    , pinnedVersion : ResourcePinState Concourse.Version Int
    , now : Maybe Time.Time
    , resourceIdentifier : Concourse.ResourceIdentifier
    , currentPage : Maybe Page
    , versions : Paginated Version
    , csrfToken : String
    , showPinBarTooltip : Bool
    , pinIconHover : Bool
    }


type alias Version =
    { id : Int
    , version : Concourse.Version
    , metadata : Concourse.Metadata
    , enabled : BoolTransitionable.BoolTransitionable
    , expanded : Bool
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
    | ToggleVersion VersionToggleAction Int
    | VersionToggled VersionToggleAction Int (Result Http.Error ())
    | PinIconHover Bool


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
                , pinnedVersion = NotPinned
                , currentPage = Nothing
                , versions =
                    { content = []
                    , pagination =
                        { previousPage = Nothing
                        , nextPage = Nothing
                        }
                    }
                , ports = ports
                , now = Nothing
                , csrfToken = flags.csrfToken
                , showPinBarTooltip = False
                , pinIconHover = False
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
        , versions =
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
                PinningTo _ ->
                    model

                _ ->
                    { model | pinnedVersion = NotPinned }

        ( Just v, True ) ->
            { model | pinnedVersion = PinnedStaticallyTo v }

        ( Just newVersion, False ) ->
            case model.pinnedVersion of
                UnpinningFrom _ ->
                    { model | pinnedVersion = UnpinningFrom newVersion }

                _ ->
                    { model | pinnedVersion = PinnedDynamicallyTo newVersion }


hasPinnedVersion : Model -> Concourse.Version -> Bool
hasPinnedVersion model v =
    case model.pinnedVersion of
        PinnedStaticallyTo pv ->
            v == pv

        PinnedDynamicallyTo pv ->
            v == pv

        UnpinningFrom pv ->
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

                versions =
                    { pagination = paginated.pagination
                    , content =
                        paginated.content
                            |> List.map
                                (\vr ->
                                    let
                                        existingVersion : Maybe Version
                                        existingVersion =
                                            model.versions.content
                                                |> List.Extra.find (\v -> v.id == vr.id)

                                        enabledStateAccordingToServer : BoolTransitionable.BoolTransitionable
                                        enabledStateAccordingToServer =
                                            if vr.enabled then
                                                BoolTransitionable.True
                                            else
                                                BoolTransitionable.False
                                    in
                                        case existingVersion of
                                            Just ev ->
                                                { ev
                                                    | enabled =
                                                        if ev.enabled == BoolTransitionable.Changing then
                                                            BoolTransitionable.Changing
                                                        else
                                                            enabledStateAccordingToServer
                                                }

                                            Nothing ->
                                                { id = vr.id
                                                , version = vr.version
                                                , metadata = vr.metadata
                                                , enabled = enabledStateAccordingToServer
                                                , expanded = False
                                                , inputTo = []
                                                , outputOf = []
                                                , showTooltip = False
                                                }
                                )
                    }

                newModel =
                    \newPage ->
                        { model
                            | versions = versions
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

                version : Maybe Version
                version =
                    model.versions.content
                        |> List.Extra.find (.id >> (==) versionID)

                newExpandedState : Bool
                newExpandedState =
                    case version of
                        Just v ->
                            not v.expanded

                        Nothing ->
                            False
            in
                ( updateVersion versionID (\v -> { v | expanded = newExpandedState }) model
                , if newExpandedState then
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
            ( updateVersion versionID (\v -> { v | inputTo = builds }) model
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
            ( updateVersion versionID (\v -> { v | outputOf = builds }) model
            , Cmd.none
            )

        NavTo url ->
            ( model, Navigation.newUrl url )

        TogglePinBarTooltip ->
            ( { model
                | showPinBarTooltip =
                    case model.pinnedVersion of
                        PinnedStaticallyTo _ ->
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
                    model.versions.content
                        |> List.Extra.find (.version >> hasPinnedVersion model)
                        |> Maybe.map .id

                newModel =
                    case ( model.pinnedVersion, pinnedVersionID ) of
                        ( PinnedStaticallyTo _, Just id ) ->
                            updateVersion id (\v -> { v | showTooltip = not v.showTooltip }) model

                        _ ->
                            model
            in
                ( newModel, Cmd.none )

        PinVersion versionID ->
            let
                version : Maybe Version
                version =
                    model.versions.content
                        |> List.Extra.find (\v -> v.id == versionID)

                cmd : Cmd Msg
                cmd =
                    case version of
                        Just v ->
                            Task.attempt VersionPinned <|
                                Concourse.Resource.pinVersion
                                    { teamName = model.resourceIdentifier.teamName
                                    , pipelineName = model.resourceIdentifier.pipelineName
                                    , resourceName = model.resourceIdentifier.resourceName
                                    , versionID = v.id
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
                pinnedVersionedResource : Maybe Version
                pinnedVersionedResource =
                    model.versions.content
                        |> List.Extra.find (.version >> hasPinnedVersion model)

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
                            model.versions.content
                                |> List.Extra.find (\v -> v.id == pinningTo)
                                |> Maybe.map .version
                        )
                        model.pinnedVersion
            in
                ( { model | pinnedVersion = newPinnedVersion }, Cmd.none )

        VersionPinned (Err _) ->
            ( { model
                | pinnedVersion = NotPinned
              }
            , Cmd.none
            )

        VersionUnpinned (Ok ()) ->
            ( { model
                | pinnedVersion = NotPinned
              }
            , Cmd.none
            )

        VersionUnpinned (Err _) ->
            ( { model
                | pinnedVersion = Pinned.quitUnpinning model.pinnedVersion
              }
            , Cmd.none
            )

        ToggleVersion action versionID ->
            ( updateVersion versionID (\v -> { v | enabled = BoolTransitionable.Changing }) model
            , Task.attempt (VersionToggled action versionID) <|
                Concourse.Resource.enableDisableVersionedResource
                    (action == Enable)
                    { teamName = model.resourceIdentifier.teamName
                    , pipelineName = model.resourceIdentifier.pipelineName
                    , resourceName = model.resourceIdentifier.resourceName
                    , versionID = versionID
                    }
                    model.csrfToken
            )

        VersionToggled action versionID result ->
            let
                newEnabledState : BoolTransitionable.BoolTransitionable
                newEnabledState =
                    case ( result, action ) of
                        ( Ok (), Enable ) ->
                            BoolTransitionable.True

                        ( Ok (), Disable ) ->
                            BoolTransitionable.False

                        ( Err _, Enable ) ->
                            BoolTransitionable.False

                        ( Err _, Disable ) ->
                            BoolTransitionable.True
            in
                ( updateVersion versionID (\v -> { v | enabled = newEnabledState }) model
                , Cmd.none
                )

        PinIconHover state ->
            ( { model | pinIconHover = state }, Cmd.none )


updateVersion : Int -> (Version -> Version) -> Model -> Model
updateVersion versionID updateFunc model =
    let
        newVersionsContent : List Version
        newVersionsContent =
            model.versions.content
                |> List.Extra.updateIf (.id >> (==) versionID) updateFunc

        versions : Paginated Version
        versions =
            model.versions
    in
        { model | versions = { versions | content = newVersionsContent } }


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
                case model.versions.pagination.previousPage of
                    Nothing ->
                        ( "btn-page-link prev disabled", Noop )

                    Just pp ->
                        ( "btn-page-link prev", LoadPage pp )

            ( nextButtonClass, nextButtonEvent ) =
                case model.versions.pagination.nextPage of
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
                    , pinBar model
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


pinBar :
    { a
        | pinnedVersion : ResourcePinState Concourse.Version Int
        , showPinBarTooltip : Bool
        , pinIconHover : Bool
    }
    -> Html Msg
pinBar { pinnedVersion, showPinBarTooltip, pinIconHover } =
    let
        pinBarVersion =
            Pinned.stable pinnedVersion

        attrList : List ( Html.Attribute Msg, Bool ) -> List (Html.Attribute Msg)
        attrList =
            List.filter Tuple.second >> List.map Tuple.first

        isPinnedStatically =
            case pinnedVersion of
                PinnedStaticallyTo _ ->
                    True

                _ ->
                    False

        isPinnedDynamically =
            case pinnedVersion of
                PinnedDynamicallyTo _ ->
                    True

                _ ->
                    False
    in
        Html.div
            (attrList
                [ ( id "pin-bar", True )
                , ( style <| Resource.Styles.pinBar { isPinned = ME.isJust pinBarVersion }, True )
                , ( onMouseEnter TogglePinBarTooltip, isPinnedStatically )
                , ( onMouseLeave TogglePinBarTooltip, isPinnedStatically )
                ]
            )
            ([ Html.div
                (attrList
                    [ ( id "pin-icon", True )
                    , ( style <|
                            Resource.Styles.pinIcon
                                { isPinned = ME.isJust pinBarVersion
                                , isPinnedDynamically = isPinnedDynamically
                                , hover = pinIconHover
                                }
                      , True
                      )
                    , ( onClick UnpinVersion, isPinnedDynamically )
                    , ( onMouseEnter <| PinIconHover True, isPinnedDynamically )
                    , ( onMouseLeave <| PinIconHover False, True )
                    ]
                )
                []
             ]
                ++ (case pinBarVersion of
                        Just v ->
                            [ viewVersion v ]

                        _ ->
                            []
                   )
                ++ (if showPinBarTooltip then
                        [ Html.div
                            [ id "pin-bar-tooltip"
                            , style Resource.Styles.pinBarTooltip
                            ]
                            [ Html.text "pinned in pipeline config" ]
                        ]
                    else
                        []
                   )
            )


checkForVersionID : Int -> Concourse.VersionedResource -> Bool
checkForVersionID versionID versionedResource =
    versionID == versionedResource.id


viewVersionedResources :
    { a
        | versions : Paginated Version
        , pinnedVersion : ResourcePinState Concourse.Version Int
    }
    -> Html Msg
viewVersionedResources { versions, pinnedVersion } =
    versions.content
        |> List.map
            (\v ->
                viewVersionedResource
                    { version = v
                    , pinnedVersion = pinnedVersion
                    }
            )
        |> Html.ul [ class "list list-collapsable list-enableDisable resource-versions" ]


viewVersionedResource :
    { version : Version
    , pinnedVersion : ResourcePinState Concourse.Version Int
    }
    -> Html Msg
viewVersionedResource { version, pinnedVersion } =
    let
        pinState =
            case Pinned.pinState version.version version.id pinnedVersion of
                PinnedStatically _ ->
                    PinnedStatically { showTooltip = version.showTooltip }

                x ->
                    x
    in
        Html.li
            (case ( pinState, version.enabled ) of
                ( Disabled, _ ) ->
                    [ style [ ( "opacity", "0.5" ) ] ]

                ( _, BoolTransitionable.False ) ->
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
                [ viewEnabledCheckbox
                    { enabled = version.enabled
                    , id = version.id
                    , pinState = pinState
                    }
                , viewPinButton
                    { versionID = version.id
                    , pinState = pinState
                    , showTooltip = version.showTooltip
                    }
                , viewVersionHeader
                    { id = version.id
                    , version = version.version
                    , pinnedState = pinState
                    }
                ]
             ]
                ++ (if version.expanded then
                        [ viewVersionBody
                            { inputTo = version.inputTo
                            , outputOf = version.outputOf
                            , metadata = version.metadata
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


viewEnabledCheckbox :
    { a
        | enabled : BoolTransitionable.BoolTransitionable
        , id : Int
        , pinState : VersionPinState
    }
    -> Html Msg
viewEnabledCheckbox { enabled, id, pinState } =
    let
        baseAttrs =
            ([ Html.Styled.Attributes.attribute "aria-label" "Toggle Resource Version Enabled"
             , css
                [ Css.marginRight <| Css.px 5
                , Css.width <| Css.px 25
                , Css.height <| Css.px 25
                , Css.float Css.left
                , Css.backgroundColor <| Css.hex "#1e1d1d"
                , Css.backgroundRepeat Css.noRepeat
                , Css.backgroundPosition2 (Css.pct 50) (Css.pct 50)
                ]
             , style [ ( "cursor", "pointer" ) ]
             ]
                ++ (case pinState of
                        PinnedStatically _ ->
                            [ style [ ( "border", "1px solid " ++ Colors.pinned ) ] ]

                        PinnedDynamically ->
                            [ style [ ( "border", "1px solid " ++ Colors.pinned ) ] ]

                        _ ->
                            []
                   )
            )
    in
        case enabled of
            BoolTransitionable.True ->
                Html.div
                    (baseAttrs
                        ++ [ style [ ( "background-image", "url(/public/images/checkmark_ic.svg)" ) ]
                           , onClick <| ToggleVersion Disable id
                           ]
                    )
                    []

            BoolTransitionable.Changing ->
                Html.div
                    (baseAttrs ++ [ style [ ( "display", "flex" ), ( "align-items", "center" ), ( "justify-content", "center" ) ] ])
                    [ Html.i [ class "fa fa-fw fa-spin fa-circle-o-notch" ] [] ]

            BoolTransitionable.False ->
                Html.div
                    (baseAttrs ++ [ onClick <| ToggleVersion Enable id ])
                    []


viewPinButton :
    { versionID : Int
    , pinState : VersionPinState
    , showTooltip : Bool
    }
    -> Html Msg
viewPinButton { versionID, pinState } =
    let
        baseAttrs =
            [ Html.Styled.Attributes.attribute "aria-label" "Pin Resource Version"
            , css
                [ Css.position Css.relative
                , Css.backgroundRepeat Css.noRepeat
                , Css.backgroundPosition2 (Css.pct 50) (Css.pct 50)
                , Css.marginRight (Css.px 5)
                , Css.width (Css.px 25)
                , Css.height (Css.px 25)
                , Css.float Css.left
                ]
            ]
    in
        case pinState of
            Enabled ->
                Html.div
                    (baseAttrs
                        ++ [ style
                                [ ( "background-color", "#1e1d1d" )
                                , ( "cursor", "pointer" )
                                , ( "background-image", "url(/public/images/pin_ic_white.svg)" )
                                ]
                           , onClick <| PinVersion versionID
                           ]
                    )
                    []

            PinnedDynamically ->
                Html.div
                    (baseAttrs
                        ++ [ style
                                [ ( "background-color", "#1e1d1d" )
                                , ( "cursor", "pointer" )
                                , ( "background-image", "url(/public/images/pin_ic_white.svg)" )
                                , ( "border", "1px solid " ++ Colors.pinned )
                                ]
                           , onClick UnpinVersion
                           ]
                    )
                    []

            PinnedStatically { showTooltip } ->
                Html.div
                    (baseAttrs
                        ++ [ style
                                [ ( "background-color", "#1e1d1d" )
                                , ( "cursor", "default" )
                                , ( "background-image", "url(/public/images/pin_ic_white.svg)" )
                                , ( "border", "1px solid " ++ Colors.pinned )
                                ]
                           , onMouseOut ToggleVersionTooltip
                           , onMouseOver ToggleVersionTooltip
                           ]
                    )
                    (if showTooltip then
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

            Disabled ->
                Html.div
                    (baseAttrs
                        ++ [ style
                                [ ( "background-color", "#1e1d1d" )
                                , ( "cursor", "default" )
                                , ( "background-image", "url(/public/images/pin_ic_white.svg)" )
                                ]
                           ]
                    )
                    []

            InTransition ->
                Html.div
                    (baseAttrs
                        ++ [ style
                                [ ( "background-color", "#1e1d1d" )
                                , ( "cursor", "default" )
                                , ( "display", "flex" )
                                , ( "align-items", "center" )
                                , ( "justify-content", "center" )
                                ]
                           ]
                    )
                    [ Html.i [ class "fa fa-fw fa-spin fa-circle-o-notch" ] [] ]


viewVersionHeader : { a | id : Int, version : Concourse.Version, pinnedState : VersionPinState } -> Html Msg
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
                    PinnedStatically _ ->
                        [ style [ ( "border", "1px solid " ++ Colors.pinned ) ] ]

                    PinnedDynamically ->
                        [ style [ ( "border", "1px solid " ++ Colors.pinned ) ] ]

                    _ ->
                        []
               )
        )
        [ viewVersion version ]


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
                (isExpanded model.versions.content)
                model.versions.content
    in
        List.concatMap
            (fetchInputAndOutputs model)
            filteredList


isExpanded : List Version -> Version -> Bool
isExpanded versions version =
    versions
        |> List.Extra.find (.id >> (==) version.id)
        |> Maybe.map .expanded
        |> Maybe.withDefault False


fetchInputAndOutputs : Model -> Version -> List (Cmd Msg)
fetchInputAndOutputs model version =
    let
        identifier =
            { teamName = model.resourceIdentifier.teamName
            , pipelineName = model.resourceIdentifier.pipelineName
            , resourceName = model.resourceIdentifier.resourceName
            , versionID = version.id
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
